package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
	"github.com/tidwall/gjson"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var (
	userName  string = "root"
	password  string = "123456"
	ipAddrees string = "127.0.0.1"
	port      int    = 3306
	dbName    string = "heco"
	charset   string = "utf8"
)

const json = `
{
    "HBTC-USDT":"0xfbe7b74623e4be82279027a286fa3a5b5280f77c",
    "ETH-USDT":"0x78c90d3f8a64474982417cdb490e840c01e516d4",
    "MDX-WHT":"0x6dd2993b50b365c707718b0807fc4e344c072ec2",
    "HUSD-BAG":"0xdde0d948b0597f08878620f1afd3070dc7243386",
    "HDOT-USDT":"0x5484ab0df3e51187f83f7f6b1a13f7a7ee98c368",
    "HBTC-BAG":"0xe468981d6fb3e8e4343350558a4ae2d4702be9e5",
    "HBO-USDT":"0xc189c6c138e78e8a4c1f1633e4c402e0c49a6049",
    "WHT-FILDA":"0x55542f696a3fecae1c937bd2e777b130587cfd2d",
    "SLNV2-USDT":"0xcf9bb6f88c5b6ddb5c067a0c6d6ae872f895b033",
    "UNI-USDT":"0x84455d880af684eb29997b82832dd71ef29c1354",
    "LINK-USDT":"0x52a342015baa2496a90a9bb6069d7692564132e6",
    "AAVE-USDT":"0xfafeaafefc5f92f22415506e78d9ab1e33c03257",
    "USDT-YFI":"0x64af3564c6d6bec5883358c560211ecd0f8d1ac7",
    "SNX-USDT":"0xc7a4c808a29fc8cd3a8a6848f7f18bed9924c692",
    "EDC-USDT":"0xfed180001c0a85a0b6d4559be5ce63330e7ca9b6",
    "BAL-USDT":"0xb6a77cdd31771a4f439622aa36b20cb53c19868c",
    "LHB-USDT":"0x023f375a51af8645d7446ba5942baedc53b0582d",
    "CAN-WHT ":"0x96e1b60bfa04bad7559802db9adda1900eb1a9d1",
    "LLC-USDT":"0x7e8af53fe202b80bb0ce383e1011a9ffb2662540",
    "SOVI-USDT":"0x7fc660a7e63956b74f22b96fef38599610799f01",
    "LLS-USDT":"0xce54ddfc0c3913ad84a0b93825754e8f954a4850",
    "HUSD-BEE":"0xe9fe4d05e4273c08b4ae99e2905847df8dfa1507"
}`

var Db *sqlx.DB

func init() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s", userName, password, ipAddrees, port, dbName, charset)
	database, err := sqlx.Open("mysql", dsn)
	if err != nil {
		fmt.Printf("mysql connect failed, detail is [%v]", err.Error())
		return
	}

	Db = database
}

func main() {
	options := []chromedp.ExecAllocatorOption{
		chromedp.Flag("headless", true), // debug
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 6.3; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/73.0.3683.103 Safari/537.36`),
	}
	options = append(chromedp.DefaultExecAllocatorOptions[:], options...)
	c, _ := chromedp.NewExecAllocator(context.Background(), options...)
	chromeCtx, cancel := chromedp.NewContext(c, chromedp.WithLogf(log.Printf))
	defer cancel()
	chromedp.Run(chromeCtx, make([]chromedp.Action, 0, 1)...)

	result := gjson.Parse(json)

	result.ForEach(func(name, token gjson.Result) bool {
		getURL(chromeCtx, token.String(), name.String())
		return true
	})
}

// Get url
func getURL(chromeCtx context.Context, token string, name string) {
	url := fmt.Sprintf("https://hecoinfo.com/token/%v", token)
	var listURL string
	var ok bool

	timeoutCtx, cancel := context.WithTimeout(chromeCtx, 3600*time.Second)
	defer cancel()

	err := chromedp.Run(timeoutCtx,
		chromedp.Tasks{
			chromedp.Navigate(url),
			chromedp.AttributeValue(`#tokentxnsiframe`, "src", &listURL, &ok),
		})
	if err != nil {
		log.Fatal(err)
	}

	// 删除页码
	listURL = listURL[:len(listURL)-4]
	getList(timeoutCtx, listURL, 1, name)
}

// 获取列表
func getList(chromeCtx context.Context, URLPath string, page int, name string) {
	listURL := fmt.Sprintf("https://hecoinfo.com/%v&p=%v", URLPath, page)
	timeoutCtx, cancel := context.WithTimeout(chromeCtx, 3600*time.Second)
	defer cancel()

	var pageStr string
	var html string
	err := chromedp.Run(timeoutCtx,
		chromedp.Tasks{
			chromedp.Navigate(listURL),
			chromedp.Text(`#maindiv > div.d-md-flex.justify-content-between.mb-4 > nav > ul > li:nth-child(3) > span > strong:nth-child(2)`, &pageStr),
			chromedp.OuterHTML(`#maindiv > div.table-responsive.mb-2.mb-md-0 > table`, &html),
		})
	if err != nil {
		log.Fatal(err)
	}

	dom, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		log.Fatal(err)
	}

	dom.Find("tbody tr").Each(func(i int, s *goquery.Selection) {
		address := s.Find("td:nth-child(6) a").Text()
		amount := s.Find("td:nth-child(7)").Text()
		fmt.Println(address, amount)
		_, err := Db.Exec("insert into transfer(name,address,balance) values(?,?,?)", name, address, amount)
		if err != nil {
			fmt.Println("Write into database failed", err)
		}
	})

	if page == 1 {
		totalPage, _ := strconv.Atoi(pageStr)
		for i := 2; i <= totalPage; i++ {
			getList(timeoutCtx, URLPath, i, name)
		}
	}
}
