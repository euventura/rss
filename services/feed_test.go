package services

import (
	"os"
	"testing"
	"time"
)

func TestFetch(t *testing.T) {

	f := newFeed()
	f.fetch()

	_, err := os.ReadFile(dir() + "/" + stPath + time.Now().Format("02012006") + "/index.html")

	if err != nil {
		t.Errorf("Erro ao ler o arquivo index.html: %v", err)
	}

}
