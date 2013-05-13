package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
)

var base_fmt = "http://maxwellinstitute.byu.edu/publications/books/?%s"
var chapter_fmt = "bookid=%s&chapid=([0-9]+)"
var header_rgx = regexp.MustCompile(`<title>(.+) by (.+)</title>`)

type Chapter struct {
	Title string
	Text  string
}

func BookData(incoming string) (string, string) {
	matches := header_rgx.FindStringSubmatch(incoming)
	if len(matches) < 3 {
		log.Fatal("Unable to parse title and author.")
	}

	title := matches[1]
	author := matches[2]

	fmt.Printf("Retrieving '%s' by %s\n", title, author)

	return title, author
}

func Chapters(book_title string, chapter_fmt string, incoming string) []Chapter {
	chapter_rgx := regexp.MustCompile(chapter_fmt)
	chapters := chapter_rgx.FindAllStringSubmatch(incoming, -1)
	chapter_data := make([]Chapter, len(chapters))

	for chapter := range chapters {
		title_fmt := fmt.Sprintf("<title>%s - (.+)</title>", book_title)
		title_rgx := regexp.MustCompile(title_fmt)

		text_rgx := regexp.MustCompile("(?s)<div id='content_readable'>(.*?)</div>")

		fmt.Printf("Retrieving chapter %d of %d", chapter+1, len(chapters))
		book_url := fmt.Sprintf(base_fmt, chapters[chapter][0])

		resp, err := http.Get(book_url)
		if err != nil {
			log.Fatal(err)
		}

		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)

		chapter_text := buf.String()
		title := title_rgx.FindStringSubmatch(chapter_text)[1]
		text := text_rgx.FindStringSubmatch(chapter_text)[1]

		fmt.Printf(" (%s)\n", title)

		new_chapter := Chapter{title, text}
		chapter_data[chapter] = new_chapter
	}

	return chapter_data
}

func AddMimetype(book_path string) {
	os.Mkdir(book_path, 0755)

	mimetype, err := os.Create(path.Join(book_path, "mimetype"))
	if err != nil {
		log.Fatal(err)
	}
	mimetype.WriteString("application/epub+zip")
	mimetype.Close()
}

func AddContainer(book_path string) {
	os.Mkdir(path.Join(book_path, "META-INF"), 0755)
	container, err := os.Create(path.Join(book_path, "META-INF", "container.xml"))
	if err != nil {
		log.Fatal(err)
	}
	container.WriteString("<?xml version=\"1.0\"?><container version=\"1.0\" xmlns=\"urn:oasis:names:tc:opendocument:xmlns:container\"><rootfiles><rootfile full-path=\"OEBPS/content.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>")
	container.Close()
}

func AddHeader(book_path string, title string, author string, url string) *os.File {
	os.Mkdir(path.Join(book_path, "OEBPS"), 0755)
	content, err := os.Create(path.Join(book_path, "OEBPS", "content.opf"))
	if err != nil {
		log.Fatal(err)
	}
	content.WriteString("<?xml version=\"1.0\"?><package version=\"2.0\" xmlns=\"http://www.idpf.org/2007/opf\" unique-identifier=\"BookId\"><metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\" xmlns:opf=\"http://www.idpf.org/2007/opf\"><dc:title>")
	content.WriteString(title)
	content.WriteString("</dc:title><dc:creator opf:role=\"aut\">")
	content.WriteString(author)
	content.WriteString("</dc:creator><dc:language>en-US</dc:language><dc:identifier id=\"BookId\">urn:uuid:")
	content.WriteString(url)
	content.WriteString("</dc:identifier></metadata>")
	return content
}

func AddContentHeader(header *os.File, chapters []Chapter) {
	toc_pre_str := "<item id=\"chapter%d\" href=\"chapter%d.xhtml\" media-type=\"application/xhtml+xml\"/>"
	toc_post_str := "<itemref idref=\"chapter%d\"/>"

	header.WriteString("<manifest><item id=\"ncx\" href=\"toc.ncx\" media-type=\"text/xml\" /><item id=\"title\" href=\"title.xhtml\" media-type=\"application/xhtml+xml\"/>")

	for chapter := range chapters {
		header.WriteString(fmt.Sprintf(toc_pre_str, chapter, chapter))
	}

	header.WriteString("</manifest><spine toc=\"ncx\"><itemref idref=\"title\"/>")

	for chapter := range chapters {
		header.WriteString(fmt.Sprintf(toc_post_str, chapter))
	}

	header.WriteString("</spine></package>")
}

func AddTOC(title string, book_str string, chapters []Chapter, book_path string) {
	toc, err := os.Create(path.Join(book_path, "OEBPS", "toc.ncx"))
	if err != nil {
		log.Fatal(err)
	}
	defer toc.Close()

	toc.WriteString(fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?><ncx xmlns=\"http://www.daisy.org/z3986/2005/ncx/\" version=\"2005-1\"><head><meta name=\"dtb:uid\" content=\"%s\"/><meta name=\"dtb:depth\" content=\"1\"/><meta name=\"dtb:totalPageCount\" content=\"0\"/><meta name=\"dtb:maxPageNumber\" content=\"0\"/></head><docTitle><text>%s</text></docTitle><navMap><navPoint id=\"title\" playOrder=\"1\"><navLabel><text>Title Page</text></navLabel><content src=\"title.xhtml\"/></navPoint>", book_str, title))

	for chapter := range chapters {
		toc.WriteString(fmt.Sprintf("<navPoint id=\"chapter%d\" playOrder=\"%d\"><navLabel><text>%s</text></navLabel><content src=\"chapter%d.xhtml\"/></navPoint>", chapter, chapter+2, chapters[chapter].Title, chapter))
	}

	toc.WriteString("</navMap></ncx>")
}

func AddTitle(book_title string, book_author string, book_path string) {
	title, err := os.Create(path.Join(book_path, "OEBPS", "title.xhtml"))
	if err != nil {
		log.Fatal(err)
	}
	defer title.Close()

	title.WriteString(fmt.Sprintf("<html>\n\t<head>\n\t\t<title>%s</title>\n\t</head>\n\t<body>\n\t\t<center><h1>%s</h1>\n\t\t<h2>by %s</h2></center>\n\t</body>\n</html>", book_title, book_title, book_author))
}

func AddChapters(chapters []Chapter, book_path string) {
	chapter_fmt := "<html>\n\t<head>\n\t\t<title>%s</title>\n\t</head>\n\t<body>\n\t\t<center><h1>%s</h1></center>\n\t\t%s\n\t</body>\n</html>"

	for chapter := range chapters {
		chapter_filename := fmt.Sprintf("chapter%d.xhtml", chapter+1)
		chapter_file, err := os.Create(path.Join(book_path, "OEBPS", chapter_filename))
		if err != nil {
			log.Fatal(err)
		}
		defer chapter_file.Close()

		chapter_file.WriteString(fmt.Sprintf(chapter_fmt, chapters[chapter].Title, chapters[chapter].Title, chapters[chapter].Text))
	}
}

func Write(title string, author string, chapters []Chapter, url string) {
	path_rgx := regexp.MustCompile(`[^a-zA-Z0-9-_.() ]`)
	sanitized_path := path_rgx.ReplaceAllLiteralString(title, " ")
	book_path := fmt.Sprintf("%s", sanitized_path)

	AddMimetype(book_path)
	AddContainer(book_path)

	header := AddHeader(book_path, title, author, url)
	defer header.Close()

	AddContentHeader(header, chapters)
	AddTOC(title, url, chapters, book_path)
	AddTitle(title, author, book_path)
	AddChapters(chapters, book_path)
}

func main() {
	fmt.Println("--")
	fmt.Println("Maxwell Institute Book to ePub Converter")
	fmt.Println("Based on GetBook.py by Matt Turner (http://guavaduck.com/)")
	fmt.Println("Go version by McKay Marston")
	fmt.Println("--")

	if len(os.Args) != 2 {
		fmt.Println("Which book?")
		os.Exit(1)
	}

	book_ref := fmt.Sprintf("bookid=%s", os.Args[1])
	book_str := fmt.Sprintf(base_fmt, book_ref)
	chapter_str := fmt.Sprintf(chapter_fmt, os.Args[1])

	resp, err := http.Get(book_str)
	if err != nil {
		log.Fatal(err)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)

	title, author := BookData(buf.String())
	chapters := Chapters(title, chapter_str, buf.String())
	Write(title, author, chapters, book_str)
}
