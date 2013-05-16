# GetFARMSEpub.go
Maxwell Institute Book to ePub Converter
Based on GetBook.py by Matt Turner (http://guavaduck.com/)
Converted to Go

## Why
I found GetBook.py in my search for F.A.R.M.S. books in epub format. But, I use Mac OS X, and tinylib is a nightmare to get installed correctly.

I like Go, but haven't had much call to use it. This seemed like a great opportunity, so I embarked on what is essentially a straight port from Python to Go.

## How
To use it, visit the book you want to download. For example: http://maxwellinstitute.byu.edu/publications/books/?bookid=105
Find the bookid at the end of the address (105).

Then simply build and run while passing in the book id.

```go run GetFARMSEpub.go 105```

## What doesn't work, yet
* there are various warnings (and errors) that could be fixed with some html validation
