package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
)

const numberOfGames = 790

var games int
var arrayCounter int
var gbRom [numberOfGames]Rom // Build an array that will have numberOfGames elements from type Rom

// Find the number of pages
func getPageNumber(page string) int {
	lastPage := 0

	// Request the HTML page.
	res, err := http.Get(page)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body div.eg-container-outer div ul.pagination.pagination-lg.justify-content-center.flex-wrap.m-0.mx-3.pb-4").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			link, ok := s.Attr("href")
			if ok {
				title := s.Text()
				if strings.Contains(title, "...") {
					sliceLink := strings.Split(link, "/")
					s := sliceLink[len(sliceLink)-2]
					// string to int
					lastPage, _ = strconv.Atoi(s)
				}
			}
		})
	})
	return lastPage
}

// CountGamesPerPage counts games per page
func CountGamesPerPage(page string) int {
	counter := 0
	// Request the HTML page.
	res, err := http.Get(page)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body div.eg-container-outer div.eg-container ul.eg-list").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			_, ok := s.Attr("href")
			if ok {
				counter++
			}
		})
	})
	return counter
}

// ExampleScrape scrapes the world as if it was a mere example
func ExampleScrape(page string) {
	// Request the HTML page.
	res, err := http.Get(page)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var downloadLink string
	retries := 0
	doc.Find("body div.eg-container-outer div.eg-container ul.eg-list").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			link, ok := s.Attr("href")
			if ok {
				title := s.Text()
				fmt.Printf("Title: %s\n", title)
				for {
					downloadLink, err = fetchDownloadLink(link)
					if err == nil || retries > 5 {
						break
					} else {
						retries++
						fmt.Println(retries, err)
					}
				}

				filename := fmt.Sprintf("%s.zip", title)
				fmt.Printf("Link: %s\nDownload: %s\nFilename: %s\n\n", link, downloadLink, filename)
			}
		})
	})
}

// FetchImageLink downlaods the image
func FetchImageLink(page string) string {
	var image string
	// Request the HTML page.
	res, err := http.Get(page)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	doc.Find("body div.eg-container.pt-0.pt-sm-3 div.eg-expand.row div.col-md-6.col-lg-3.px-3 div.mb-3 picture").Each(func(i int, s *goquery.Selection) {
		s.Find("img").Each(func(i int, s *goquery.Selection) {
			link, ok := s.Attr("src")
			if ok {
				image = link
			}
		})
	})
	return image
}

func downloadFile(filepath string, url string) (err error) {
	defer cleanup()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func fetchDownloadLink(url string) (string, error) {
	// create chrome instance
	ctx, cancel := chromedp.NewContext(context.Background(), chromedp.WithLogf(log.Printf))
	defer cancel()

	// create a timeout
	ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// navigate to a page, wait for an element, click
	var example string
	var staticRomLink string
	var res string
	var exists bool
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Text(`body > div.eg-container.pt-0.pt-sm-3 > div.eg-expand.row > div.col-md-6.col-lg-5.px-3.px-md-0.mb-4 > div.eg-meta.mb-2`, &example),
		chromedp.Click(`body > div.eg-container.pt-0.pt-sm-3 > div.eg-expand.row > div.col-md-12.col-lg-4.px-3.mb-3 > form:nth-child(1) > button`, chromedp.NodeVisible),
		chromedp.Sleep(10*time.Second),
		chromedp.Text(`#eg-notify`, &example),
		chromedp.AttributeValue("body > iframe:nth-child(14)", "src", &staticRomLink, &exists),
		chromedp.InnerHTML("body", &res),
	)
	if err != nil {
		return staticRomLink, err
	}

	if !exists {
		// Workfloa with InnerHTML
		linkZip := strings.Split(res, "style=\"display:none;\" src=\"")
		linkZip2 := strings.Split(linkZip[1], "\">")
		staticRomLink = linkZip2[0]
		if staticRomLink == "" {
			err = fmt.Errorf("no iframe. I will retry")
		}
	}
	return staticRomLink, err
}

func fetchAllGames(consolePage string) int {
	var games int
	lastpage := getPageNumber(consolePage)
	var link string
	for i := 1; i <= lastpage; i++ {
		if i == 1 {
			link = consolePage
		} else {
			link = fmt.Sprintf("%s%d/", consolePage, i)
		}
		games += CountGamesPerPage(link)
	}
	return games
}

// Rom defines a typical game card
// has to start with capital letter or the Marshal won't work in JSON
type Rom struct {
	Title        string `json:"title"`
	Link         string `json:"link"`
	DownloadLink string `json:"download_link"`
	Filename     string `json:"filename"`
	Image        string `json:"image"`
	Region       string `json:"region"`
	Quality      string `json:"quality"`
	Hack         string `json:"hack"`
	Gameboy      string `json:"gameboy"`
}

var wg sync.WaitGroup

func cleanup() {
	defer wg.Done()
	if r := recover(); r != nil {
		fmt.Println("Recovered in cleanup", r)
	}
}

func downloadRoms(gbRom *[numberOfGames]Rom) {
	for _, v := range gbRom {
		if v.Quality == "Verified" && v.Gameboy != "Bung Fix" && v.Gameboy != "Color" && v.Hack == "No" && (strings.Contains(v.Region, "USA") || strings.Contains(v.Region, "Europe")) {
			fmt.Printf("%s\n", v.Title)
			// fetch rom
			wg.Add(1)
			go downloadFile(v.Filename, v.DownloadLink)

			// fetch logo image
			imgExtension := filepath.Ext(v.Image)
			filenameImage := fmt.Sprintf("%s%s", v.Title, imgExtension)
			wg.Add(1)
			go downloadFile(filenameImage, v.Image)
		}
	}
	wg.Wait()
}

func searchGame(gbRom *[numberOfGames]Rom, game string) {
	fmt.Printf("Searching %s ... \n\n", game)
	for _, v := range gbRom {
		if strings.Contains(v.Title, game) {
			println("Title: " + v.Title)
			println("Gameboy: " + v.Gameboy)
			println("Region: " + v.Region)
			println("Quality: " + v.Quality)
			fmt.Printf("Cheats: %s\n\n", v.Hack)
		}
	}
}

func loopGames(gbRom *[numberOfGames]Rom, page string) {
	// Request the HTML page.
	res, err := http.Get(page)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	var downloadLink string
	region := "Unknown"
	quality := "Unknown"
	hack := "Unknown"
	gameboy := "Unknown"
	retries := 0
	doc.Find("body div.eg-container-outer div.eg-container ul.eg-list").Each(func(i int, s *goquery.Selection) {
		s.Find("a").Each(func(i int, s *goquery.Selection) {
			link, ok := s.Attr("href")
			if ok {
				title := s.Text()
				fmt.Printf("Array[%d] - Game %d/%d\n", arrayCounter, arrayCounter+1, numberOfGames)
				fmt.Printf("Title: %s\n", title)
				for {
					downloadLink, err = fetchDownloadLink(link)
					if err == nil || retries > 5 {
						break
					} else {
						retries++
						fmt.Println(retries, err)
					}
				}

				// TODO: Do not assume it's always a zip file extension
				extension := filepath.Ext(downloadLink)
				filename := fmt.Sprintf("%s%s", title, extension)

				image := FetchImageLink(link)
				extension = filepath.Ext(image)
				// filenameImage := fmt.Sprintf("%s%s", title, extension)

				if strings.Contains(downloadLink, "(E)") {
					region = "Europe"
				} else if strings.Contains(downloadLink, "(U)") {
					region = "USA"
				} else if strings.Contains(downloadLink, "(J)") || strings.Contains(downloadLink, "[J]") {
					region = "Japan"
				} else if strings.Contains(downloadLink, "(G)") {
					region = "Germany"
				} else if strings.Contains(downloadLink, "(UE)") || strings.Contains(downloadLink, "(EU)") || strings.Contains(downloadLink, "(U)(E)") {
					region = "USA/Europe"
				} else if strings.Contains(downloadLink, "(JU)") {
					region = "Japan/USA"
				} else if strings.Contains(downloadLink, "(JE)") {
					region = "Japan/Europe"
				} else if strings.Contains(downloadLink, "(JUE)") {
					region = "Japan/USA/Europe"
				} else if strings.Contains(downloadLink, "(1)") {
					region = "Japan/Korea"
				} else if strings.Contains(downloadLink, "(4)") {
					region = "USA/Brazil (NTSC)"
				} else if strings.Contains(downloadLink, "(A)") {
					region = "Australia"
				} else if strings.Contains(downloadLink, "(B)") {
					region = "Brazil"
				} else if strings.Contains(downloadLink, "(K)") {
					region = "Korea"
				} else if strings.Contains(downloadLink, "(C)") {
					region = "China"
				} else if strings.Contains(downloadLink, "(NL)") {
					region = "Netherlands"
				} else if strings.Contains(downloadLink, "(PD)") {
					region = "Public Domain"
				} else if strings.Contains(downloadLink, "(F)") {
					region = "France"
				} else if strings.Contains(downloadLink, "(S)") {
					region = "Spain"
				} else if strings.Contains(downloadLink, "(FC)") {
					region = "France/Canada"
				} else if strings.Contains(downloadLink, "(SW)") {
					region = "Sweden"
				} else if strings.Contains(downloadLink, "(FN)") {
					region = "Finland"
				} else if strings.Contains(downloadLink, "(UK)") {
					region = "England"
				} else if strings.Contains(downloadLink, "(GR)") {
					region = "Greece"
				} else if strings.Contains(downloadLink, "I") {
					region = "Italy"
				} else if strings.Contains(downloadLink, "(HK)") {
					region = "Hong Kong"
				} else if strings.Contains(downloadLink, "(H)") {
					region = "Netherlands/Holland"
				} else {
					region = "Unknown"
				}

				if strings.Contains(downloadLink, "[b]") || strings.Contains(downloadLink, "[B]") {
					quality = "Bad Dump (crappy port / buggy)"
				} else if strings.Contains(downloadLink, "[f]") {
					quality = "Fixed rom from a previously bad port"
				} else if strings.Contains(downloadLink, "[f]") {
					quality = "Fixed rom from a previously bad port"
				} else if strings.Contains(downloadLink, "[!]") {
					quality = "Verified"
				} else if strings.Contains(downloadLink, "[o]") {
					quality = "Overdump - sometimes bad"
				} else {
					quality = "Unknown"
				}

				if strings.Contains(downloadLink, "[h]") || strings.Contains(downloadLink, "Hack") || strings.Contains(downloadLink, "hack") || strings.Contains(downloadLink, "[p]") || strings.Contains(downloadLink, "[t]") {
					hack = "Hack/Pirate or trainer (code that gets executed before the game is begun. It allows cheats from the menu)"
				} else {
					hack = "No"
				}

				if strings.Contains(downloadLink, "[C]") {
					gameboy = "Color"
				} else if strings.Contains(downloadLink, "[S]") {
					gameboy = "Super"
				} else if strings.Contains(downloadLink, "[BF]") {
					gameboy = "Bung Fix"
				} else {
					gameboy = "Classic"
				}

				gbRom[arrayCounter].Title = title
				gbRom[arrayCounter].Link = link
				gbRom[arrayCounter].DownloadLink = downloadLink
				gbRom[arrayCounter].Filename = filename
				gbRom[arrayCounter].Image = image
				gbRom[arrayCounter].Region = region
				gbRom[arrayCounter].Quality = quality
				gbRom[arrayCounter].Hack = hack
				gbRom[arrayCounter].Gameboy = gameboy

				fmt.Printf("Link: %s\nDownload: %s\nFilename: %s\nImage: %s\nRegion: %s\nQuality: %s\nHack: %s\nConsole: %s\n\n", link, downloadLink, filename, image, region, quality, hack, gameboy)

				arrayCounter++
			}
		})
	})
}

// FileExists reports whether the named file or directory exists.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func main() {
	// Find the homedir and create the file
	dbFileName := "db.json"
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("couldn't find the $HOME directory\nError: %s", err)

	}
	dbFile := home + "/" + dbFileName

	// If the file exists parse it
	if FileExists(dbFile) {
		fmt.Println("Parsing the file")
		// Read the file
		fileJSON, err := ioutil.ReadFile(dbFile)
		if err != nil {
			log.Fatalf("Could not read the file.\nError: %s\n", err)
		}
		json.Unmarshal(fileJSON, &gbRom)

		// Download the games
		// searchGame(&gbRom, "Luke")
		downloadRoms(&gbRom)
	} else {
		// If it doesn't exist create it

		fmt.Printf("No local file found. Downloading database ... %s\n", dbFile)

		// Start the HTML scraping
		gameBoyPage := "https://www.emulatorgames.net/roms/gameboy/"
		games = fetchAllGames(gameBoyPage)
		fmt.Printf("There are %d roms for gameboy\n\n", games)

		// If this number has been changed, open a pull-request
		if games != numberOfGames {
			err := fmt.Errorf("the actual number of games is %d but the program knows %d. Open a github issue with this information", games, numberOfGames)
			log.Fatal(err)
		}

		var link string
		lastpage := getPageNumber(gameBoyPage)
		for i := 1; i <= lastpage; i++ {
			if i == 1 {
				link = "https://www.emulatorgames.net/roms/gameboy/"
			} else {
				link = fmt.Sprintf("%s%d/", gameBoyPage, i)
			}
			loopGames(&gbRom, link) // Pass the array through pointers so the changes pass back to main
		}

		// Read and print the array
		// for k, v := range gbRom {
		// 	fmt.Printf("The game %d is %s.\n", k, v)
		// }

		// Write the structure in a local file
		fileJSON, err := json.Marshal(gbRom)
		if err != nil {
			log.Fatal("Couldn't encode to JSON")
		}

		//fmt.Fprintf(os.Stdout, "%s", fileJSON)

		err = ioutil.WriteFile(dbFile, fileJSON, 0644)
		if err != nil {
			log.Fatalf("Couldn't update the db file %s\nError: %s", dbFile, err)
		}

		fmt.Println("Phew .... It's finished")
	}

}
