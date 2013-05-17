/* Copyright (c) 2013, McKay Marston <mckay.marston@greymanlabs.com>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of the Grey Man Labs, LLC nor the
      names of its contributors may be used to endorse or promote products
      derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL MCKAY MARSTON BE LIABLE FOR ANY
DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE. */

package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
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

func BookData(resp *http.Response) (string, string, string) {
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	body := buf.String()

	matches := header_rgx.FindStringSubmatch(body)
	if len(matches) < 3 {
		log.Fatal("Unable to parse title and author.")
	}

	title := matches[1]
	author := matches[2]

	fmt.Printf("Retrieving '%s' by %s\n", title, author)

	return title, author, body
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

func AddMimetype(zippy *zip.Writer) {
	mimetype, err := zippy.Create("mimetype")
	if err != nil {
		log.Fatal(err)
	}
	mimetype.Write([]byte("application/epub+zip"))
}

func AddContainer(zippy *zip.Writer) {
	container, err := zippy.Create(path.Join("META-INF", "container.xml"))
	if err != nil {
		log.Fatal(err)
	}
	container.Write([]byte("<?xml version=\"1.0\"?><container version=\"1.0\" xmlns=\"urn:oasis:names:tc:opendocument:xmlns:container\"><rootfiles><rootfile full-path=\"OEBPS/content.opf\" media-type=\"application/oebps-package+xml\"/></rootfiles></container>"))
}

func AddHeader(title string, author string, url string, chapters []Chapter, zippy *zip.Writer) {
	content, err := zippy.Create(path.Join("OEBPS", "content.opf"))
	if err != nil {
		log.Fatal(err)
	}

	content.Write([]byte("<?xml version=\"1.0\"?><package version=\"2.0\" xmlns=\"http://www.idpf.org/2007/opf\" unique-identifier=\"BookId\"><metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\" xmlns:opf=\"http://www.idpf.org/2007/opf\"><dc:title>"))
	content.Write([]byte(title))
	content.Write([]byte("</dc:title><dc:creator opf:role=\"aut\">"))
	content.Write([]byte(author))
	content.Write([]byte("</dc:creator><dc:language>en-US</dc:language><dc:identifier id=\"BookId\">urn:uuid:"))
	content.Write([]byte(url))
	content.Write([]byte("</dc:identifier></metadata>"))

	AddContentHeader(content, chapters)
}

func AddContentHeader(header io.Writer, chapters []Chapter) {
	toc_pre_str := "<item id=\"chapter%d\" href=\"chapter%d.xhtml\" media-type=\"application/xhtml+xml\"/>"
	toc_post_str := "<itemref idref=\"chapter%d\"/>"

	header.Write([]byte("<manifest><item id=\"ncx\" href=\"toc.ncx\" media-type=\"text/xml\" /><item id=\"title\" href=\"title.xhtml\" media-type=\"application/xhtml+xml\"/>"))

	for chapter := range chapters {
		header.Write([]byte(fmt.Sprintf(toc_pre_str, chapter, chapter)))
	}

	header.Write([]byte("</manifest><spine toc=\"ncx\"><itemref idref=\"title\"/>"))

	for chapter := range chapters {
		header.Write([]byte(fmt.Sprintf(toc_post_str, chapter)))
	}

	header.Write([]byte("</spine></package>"))
}

func AddTOC(title string, book_str string, chapters []Chapter, zippy *zip.Writer) {
	toc, err := zippy.Create(path.Join("OEBPS", "toc.ncx"))
	if err != nil {
		log.Fatal(err)
	}

	toc.Write([]byte(fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?><ncx xmlns=\"http://www.daisy.org/z3986/2005/ncx/\" version=\"2005-1\"><head><meta name=\"dtb:uid\" content=\"%s\"/><meta name=\"dtb:depth\" content=\"1\"/><meta name=\"dtb:totalPageCount\" content=\"0\"/><meta name=\"dtb:maxPageNumber\" content=\"0\"/></head><docTitle><text>%s</text></docTitle><navMap><navPoint id=\"title\" playOrder=\"1\"><navLabel><text>Title Page</text></navLabel><content src=\"title.xhtml\"/></navPoint>", book_str, title)))

	for chapter := range chapters {
		toc.Write([]byte(fmt.Sprintf("<navPoint id=\"chapter%d\" playOrder=\"%d\"><navLabel><text>%s</text></navLabel><content src=\"chapter%d.xhtml\"/></navPoint>", chapter, chapter+2, chapters[chapter].Title, chapter)))
	}

	toc.Write([]byte("</navMap></ncx>"))
}

func AddTitle(book_title string, book_author string, zippy *zip.Writer) {
	title, err := zippy.Create(path.Join("OEBPS", "title.xhtml"))
	if err != nil {
		log.Fatal(err)
	}

	title.Write([]byte(fmt.Sprintf("<html>\n\t<head>\n\t\t<title>%s</title>\n\t</head>\n\t<body>\n\t\t<center><h1>%s</h1>\n\t\t<h2>by %s</h2></center>\n\t</body>\n</html>", book_title, book_title, book_author)))
}

func AddChapters(chapters []Chapter, zippy *zip.Writer) {
	chapter_fmt := "<html>\n\t<head>\n\t\t<title>%s</title>\n\t</head>\n\t<body>\n\t\t<center><h1>%s</h1></center>\n\t\t%s\n\t</body>\n</html>"

	for chapter := range chapters {
		chapter_filename := fmt.Sprintf("chapter%d.xhtml", chapter+1)
		chapter_file, err := zippy.Create(path.Join("OEBPS", chapter_filename))
		if err != nil {
			log.Fatal(err)
		}

		chapter_file.Write([]byte(fmt.Sprintf(chapter_fmt, chapters[chapter].Title, chapters[chapter].Title, chapters[chapter].Text)))
	}
}

func Write(title string, author string, chapters []Chapter, url string) {
	path_rgx := regexp.MustCompile(`[^a-zA-Z0-9-_.() ]`)
	sanitized_path := path_rgx.ReplaceAllLiteralString(title, " ")
	book_path := fmt.Sprintf("%s", sanitized_path)

	epub_name := fmt.Sprintf("%s.epub", book_path)
	epub, err := os.Create(epub_name)
	if err != nil {
		log.Fatal(err)
	}
	defer epub.Close()

	zippy := zip.NewWriter(epub)

	AddMimetype(zippy)
	AddContainer(zippy)
	AddHeader(title, author, url, chapters, zippy)
	AddTOC(title, url, chapters, zippy)
	AddTitle(title, author, zippy)
	AddChapters(chapters, zippy)

	err = zippy.Close()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Wrote '%s'\n", epub_name)
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

	title, author, body := BookData(resp)
	chapters := Chapters(title, chapter_str, body)
	Write(title, author, chapters, book_str)
}
