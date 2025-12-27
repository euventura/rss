package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gosimple/slug"
	"github.com/mmcdole/gofeed"
)

type Feed struct {
	Title   string
	Sources []Source
}

type Source struct {
	URL  string
	Star bool
}

type Entry struct {
	Star        bool
	Title       string
	Url         string
	Link        string
	Author      string
	Content     template.HTML
	Description string
	Date        time.Time
	Class       string
	ID          string
	Menu        []string
}

var artPath = "/template/article.html"
var hePath = "/template/headline.html"
var mePath = "/template/menu.html"

var stPath = "./static/editions/"

func newFeed() *Feed {
	return &Feed{
		Title:   "RSS Zombie",
		Sources: []Source{},
	}

}

func main() {
	fmt.Println("Start")
	f := newFeed()
	f.fetch()
	fmt.Println("Success")

}

func (f *Feed) loadSources() {

	file, err := os.Open("./sources.txt")

	if err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			star := false
			lastChar := line[len(line)-1:]
			if lastChar == "*" {
				line = line[:len(line)-1]
				star = true
			}
			source := Source{
				URL:  line,
				Star: star,
			}
			f.Sources = append(f.Sources, source)
		}
		return
	}

}

func (f *Feed) fetch() {
	f.loadSources()
	nPath := stPath + time.Now().Format("02012006")
	err := os.MkdirAll(nPath, os.ModePerm)
	if err != nil {
		fmt.Println("Erro ao criar diretório:", err)
		return
	}

	fp := gofeed.NewParser()
	ch := make(chan string, len(f.Sources)) // Buffer para evitar deadlock
	wg := sync.WaitGroup{}

	for _, source := range f.Sources {
		fmt.Println("Start: " + source.URL)
		fp.UserAgent = "CloudFair 0.1"
		feed, err := fp.ParseURL(source.URL)

		if err != nil {
			fmt.Println("Erro ao buscar feed:", err)
			continue
		}
		wg.Add(1)
		go f.process(feed, source.Star, nPath, ch, &wg)
	}

	// Aguarda todas as goroutines terminarem e fecha o canal
	go func() {
		wg.Wait()
		close(ch)
	}()

	var index string

	// Aguarda todos os resultados do canal
	for result := range ch {
		index += result
	}

	fmt.Printf("Writing Index.html: %d\n", len(index))
	os.WriteFile(nPath+"/index.html", []byte(index), 0644)
	f.makeMenu()
}

func dir() string {
	dir, _ := os.Getwd()
	return dir
}

func (f *Feed) process(gof *gofeed.Feed, star bool, wPath string, ch chan<- string, wg *sync.WaitGroup) {
	var headline string
	var fiName string
	defer wg.Done()
	fmt.Printf("Items: %d", len(gof.Items))
	for _, item := range gof.Items {

		author := item.Authors[0].Name

		if author == "" {
			url, _ := url.Parse(item.Link)
			author = url.Hostname()
		}

		fmt.Println("Author", author)

		const regex = `<.*?>`
		r := regexp.MustCompile(regex)
		desc := r.ReplaceAllString(item.Description, "")
		words := strings.Fields(desc)

		class := slug.Make(author)

		url := "/editions/" + time.Now().Format("02012006") + "/" + slug.Make(item.Title) + ".html"

		data := Entry{
			Star:        star,
			Title:       item.Title,
			Link:        item.Link,
			Url:         url,
			Author:      author,
			Content:     template.HTML(item.Content),
			Date:        *item.PublishedParsed,
			Description: strings.Join(words[0:min(38, len(words))], " "),
			Class:       class,
			ID:          slug.Make(item.Title),
		}
		fiName = slug.Make(item.Title) + ".html"
		article := f.make(data, dir()+artPath)
		err := os.WriteFile(wPath+"/"+fiName, []byte(article), 0644)
		if err != nil {
			fmt.Println("Erro ao escrever arquivo:", err)
			return
		}
		fmt.Println("Written:", fiName)
		headline += f.make(data, dir()+hePath)
	}

	ch <- headline
}

func (f *Feed) make(data Entry, templatePath string) string {

	tmp, err := os.ReadFile(templatePath)

	if err != nil {
		panic(err)
	}

	buf := &bytes.Buffer{}

	t, err := template.New(templatePath).Parse(string(tmp))
	err = t.ExecuteTemplate(buf, templatePath, data)

	if err != nil {
		panic(err)
	}

	return buf.String()
}

func (f *Feed) makeMenu() {
	di, err := os.ReadDir(stPath)

	if err != nil {
		fmt.Println("Erro ao ler diretório:", err)
		return
	}

	e := Entry{}

	for _, d := range di {
		if (d.IsDir()) && (len(d.Name()) == 8) {
			{
				e.Menu = append(e.Menu, d.Name())
				continue
			}
		}
	}
	mePAthC := dir() + mePath

	cont := f.make(e, mePAthC)

	wPath := stPath + "/menu.html"
	err = os.WriteFile(wPath, []byte(cont), 0644)

}
