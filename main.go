package main

import (
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
	Back        string
}

var artPath = "/template/article.html"
var hePath = "/template/headline.html"
var indPath = "/template/index.html"
var setupOnce sync.Once

func prepareDocs() {

	_ = os.MkdirAll("./docs", os.ModePerm)
	_ = os.RemoveAll("./docs/y")

	entries, err := os.ReadDir("./docs")
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".html") {
			os.Remove("./docs" + e.Name())
		}
	}
}

func newFeed() *Feed {
	return &Feed{
		Title:   "RSS",
		Sources: []Source{},
	}

}

func main() {

	f := newFeed()
	fmt.Println("Fetching feeds...")

	f.fetch()
	fmt.Println("Success")

}

func (f *Feed) loadSources() {

	path := dir() + "/sources.txt"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Erro ao ler sources.txt:", err)
		return
	}

	sources := strings.Split(string(data), "\n")

	for _, raw := range sources {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

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

}

func (f *Feed) fetch() {
	f.loadSources()

	setupOnce.Do(prepareDocs)

	outDir := "./docs"
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		fmt.Println("Erro ao criar diretório:", err)
		return
	}

	fp := gofeed.NewParser()
	ch := make(chan string, len(f.Sources)) // Buffer para evitar deadlock
	wg := sync.WaitGroup{}

	for _, source := range f.Sources {
		fmt.Println("Start: " + source.URL)
		fp.UserAgent = "Euventura Rss 0.1"
		feed, err := fp.ParseURL(source.URL)

		if err != nil {
			fmt.Println("Erro ao buscar feed:", err)
			continue
		}
		wg.Add(1)
		go f.process(feed, source.Star, outDir, ch, &wg)
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
	indTpl := f.make(Entry{Content: template.HTML(index)}, dir()+indPath)
	os.WriteFile(outDir+"/index.html", []byte(indTpl), 0644)
}

func dir() string {
	dir, _ := os.Getwd()
	return dir
}

func (f *Feed) process(gof *gofeed.Feed, star bool, wPath string, ch chan<- string, wg *sync.WaitGroup) {
	var headline string
	var fiName string
	defer wg.Done()

	setupOnce.Do(prepareDocs)

	fmt.Printf("Items: %d", len(gof.Items))
	for _, item := range gof.Items {

		if item.PublishedParsed.Format("02012006") != time.Now().AddDate(0, 0, -1).Format("02012006") {
			continue
		}

		author := item.Authors[0].Name

		if author == "" {
			url, _ := url.Parse(item.Link)
			author = url.Hostname()
		}

		fmt.Println("Author", author)

		// Prefer full content; fall back to description when content is empty
		contentHTML := item.Content
		if contentHTML == "" {
			contentHTML = item.Description
		}

		const regex = `<.*?>`
		r := regexp.MustCompile(regex)
		desc := r.ReplaceAllString(item.Description, "")

		if desc == "" {
			desc = r.ReplaceAllString(contentHTML, "")
		}

		words := strings.Fields(desc)

		class := slug.Make(author)
		back := "/rss/"
		url := back + slug.Make(item.Title) + ".html"

		data := Entry{
			Star:        star,
			Title:       item.Title,
			Link:        item.Link,
			Url:         url,
			Author:      author,
			Content:     template.HTML(contentHTML),
			Date:        *item.PublishedParsed,
			Description: strings.Join(words[0:min(38, len(words))], " "),
			Class:       class,
			ID:          slug.Make(item.Title),
			Back:        back + "index.html",
		}

		fiName = slug.Make(item.Title) + ".html"
		article := f.make(data, dir()+artPath)
		article = f.make(Entry{Content: template.HTML(article)}, dir()+indPath)
		// Always write to ./docs per new flow
		err := os.WriteFile("./docs/"+fiName, []byte(article), 0644)
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
