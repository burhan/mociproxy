package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var sticket = regexp.MustCompile(`id="libirary"\s+value="(.*?)"\s+/>`)
var paciid = regexp.MustCompile(`"AddressAutoNo":"(.*?)"`)

func getData(cids chan string, results chan []string, wg *sync.WaitGroup) {
	defer wg.Done()
	// Initiate a session and get the cookies
	for cid := range cids {

		cookieJar, _ := cookiejar.New(nil)
		client := &http.Client{
			Jar: cookieJar,
		}
		resp, _ := client.Get("https://www.moci.shop/Associations/WebPages/index.aspx")

		// Get the "Sticket" value
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			panic(err)

		}
		bodyString := string(bodyBytes)
		sticketvalue := sticket.FindStringSubmatch(bodyString)[1]

		// P_Civil_Serial accepts any numerical value, we pass the civil id again
		body := strings.NewReader(fmt.Sprintf("{'CivilID':'%s','P_CIVIL_SERIAL':'%s'}", cid, cid))
		req, err := http.NewRequest("POST", "https://www.moci.shop/Associations/BusinessLayer/WebMethods.asmx/GetCivilName", body)
		if err != nil {
			panic(err)
		}
		req.Header.Set("Authority", "www.moci.shop")
		req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
		req.Header.Set("Dnt", "1")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Sticket", sticketvalue)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36")
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Origin", "https://www.moci.shop")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Referer", "https://www.moci.shop/Associations/WebPages/index.aspx")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9,ar-KW;q=0.8,ar;q=0.7,ur-PK;q=0.6,ur;q=0.5")

		resp, err = client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			bodyBytes, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				panic(err)
			}
			bodyString = string(bodyBytes)
			temp := paciid.FindStringSubmatch(bodyString)
			if len(temp) > 0 {
				results <- []string{cid, paciid.FindStringSubmatch(bodyString)[1]}
			} else {
				results <- []string{cid, "No result found"}
			}
		} else {
			results <- []string{cid, "Error fetching data"}
		}
	}
}

func main() {

	wg := new(sync.WaitGroup)
	wg.Add(1)
	filename := flag.String("input", "", "a csv file, with one column, no headers. The first column is the civil id")
	flag.Parse()
	if *filename == "" {
		log.Fatal("Provide a file to parse")
	}
	cids := make(chan string)
	readerr := make(chan error)
	results := make(chan []string)

	ts := time.Now()

	file, err := os.Create(fmt.Sprintf("%s_data_extract.csv", ts.Format("20060102150405")))
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	writer := csv.NewWriter(file)

	writer.Comma = ','
	writer.Write([]string{"Civil ID", "Result"})
	defer writer.Flush()

	go func(filename string, cids chan string, readerr chan error) {

		csvfile, err := os.Open(filename)

		if err != nil {
			log.Fatalln(err)
		}
		defer csvfile.Close()
		r := csv.NewReader(csvfile)
		for {
			record, err := r.Read()

			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			cids <- record[0]

		}
		close(cids)
		readerr <- err
	}(*filename, cids, readerr)

	go getData(cids, results, wg)

	go func() {
		wg.Wait()
		close(results)
	}()

	defer writer.Flush()

	for i := range results {
		writer.Write(i)
	}

	log.Println("Done")

}
