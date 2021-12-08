package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type Download struct {
	Url           string
	TargetPath    string
	TotalSections int
}

func (d Download) getNewRequest(method string) (*http.Request, error) {
	r, err := http.NewRequest(method, d.Url, nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/53.0.2785.143 Safari/537.36")
	return r, nil
}

func (d Download) Download() error {
	fmt.Println("Making Connection")

	r, err := d.getNewRequest("HEAD")
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	fmt.Printf("Got response: %s\n", resp.Status)
	fmt.Println(resp)
	if resp.StatusCode > 299 {
		return errors.New(fmt.Sprintf("Can't process, response is %v", resp.StatusCode))
	}
	size, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		return err
	}
	fmt.Printf("Size: %d\n", size)
	sections := make([][2]int, d.TotalSections)
	eachSize := size / d.TotalSections
	// example: if size is 100, our section should like:
	// [[0 10] [11 21] [22 32] [33 43] [44 54] [55 65] [66 76] [77 87] [88 98] [99 99]]
	var wg sync.WaitGroup
	for i := range sections {
		if i == 0 {
			// starting byte of first section
			sections[i][0] = 0
		} else {
			sections[i][0] = sections[i-1][1] + 1
		}
		if i < d.TotalSections-1 {
			// ending byte of current section
			sections[i][1] = sections[i][0] + eachSize
		} else {
			// ending byte of last section
			sections[i][1] = size
		}
	}
	fmt.Println(sections)
	for i, s := range sections {
		wg.Add(1)
		go func(i int, s [2]int) {
			err = d.downloadSection(i, s)
			if err != nil {
				panic(err)
			}
			defer wg.Done()
		}(i, s)
	}
	wg.Wait()
	d.mergeFiles(sections)
	return nil
}

func (d Download) mergeFiles(sections [][2]int) error {
	f, err := os.OpenFile(d.TargetPath, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	for i := range sections {
		b, err := ioutil.ReadFile(fmt.Sprintf("section-%v.tmp", i))
		if err != nil {
			return err
		}
		n, err := f.Write(b)
		if err != nil {
			return err
		}
		fmt.Printf("Wrote %v bytes for section %v\n", n, i)
	}
	return nil
}

func (d Download) downloadSection(i int, s [2]int) error {
	r, err := d.getNewRequest("GET")
	if err != nil {
		return err
	}
	r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", s[0], s[1]))
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fmt.Sprintf("section-%v.tmp", i), b, os.ModePerm)
	if err != nil {
		return err
	}
	fmt.Printf("Downloaded %v bytes for section %v: %v \n", resp.Header.Get("Content-Length"), i, s)

	return nil
}

func main() {
	startTime := time.Now()
	d := Download{
		Url:           "https://www.dropbox.com/s/lgvhj/sample.mp4?dl=1",
		TargetPath:    "./sample.mp4",
		TotalSections: 10,
	}
	err := d.Download()
	if err != nil {
		log.Fatalf("Error: %s", err)
	}
	endTime := time.Now()
	fmt.Sprintf("Time taken: %f", endTime.Sub(startTime).Seconds())
}
