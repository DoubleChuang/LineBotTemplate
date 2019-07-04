// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DoubleChuang/gogrs/twse"
	"github.com/DoubleChuang/gogrs/utils"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/pkg/errors"
	"github.com/toomore/gogrs/tradingdays"
)

const shortForm = "20060102"

var defaultB bool = false
var useMtss *bool = &defaultB
var useT38 *bool = &defaultB
var useT44 *bool = &defaultB
var useMa *bool = &defaultB
var useCp *bool = &defaultB
var defaultDate string = "20190703"
var useDate *string = &defaultDate
var bot *linebot.Client

func main() {
	var err error
	go getTWSE("ALLBUT0999", 3)

	//bot, err = linebot.New(ChannelSecret, ChannelAccessToken)
	bot, err = linebot.New(os.Getenv("ChannelSecret"), os.Getenv("ChannelAccessToken"))
	log.Println("Bot:", bot, " err:", err)
	http.HandleFunc("/callback", callbackHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
	})
	port := os.Getenv("PORT")
	addr := fmt.Sprintf(":%s", port)
	//addr := fmt.Sprintf(":%s", PORT)
	//http.ListenAndServe(addr, nil)
	err = http.ListenAndServeTLS(addr, "./CFile/doublechuang.nctu.me.crt", "./CFile/doublechuang.nctu.me.key", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
func prepareStock(stock *twse.Data, mindata int) error {

	if _, err := stock.Get(); err != nil {
		return err
	}

	if stock.Len() < mindata {
		start := stock.Len()
		for {
			stock.PlusData()
			if stock.Len() > mindata {
				break
			}
			if stock.Len() == start {
				break
			}
			start = stock.Len()
		}
		if stock.Len() < mindata {
			return errors.New("Can't prepare enough data, please check file has data or remove cache file")
		}
	}
	return nil
}

type resData struct {
	todayRange float64
	todayPrice float64
	todayGain  float64
	NDayAvg    float64
	overMA     bool
}

func showStock(stock *twse.Data, minDataNum int) (*resData, error) {
	var todayRange float64
	var todayPrice float64
	res := new(resData)
	minData := minDataNum
	if len(stock.RawData) < minData {
		fmt.Println(stock.Name, "No Data")
		return nil, errors.New("No Data")
	}
	rangeList := stock.GetRangeList()
	priceList := stock.GetPriceList()
	if len(rangeList) >= minData && len(priceList) >= minData {
		todayRange = rangeList[len(rangeList)-1]
		todayPrice = priceList[len(priceList)-1]
		res.todayRange = todayRange
		res.todayPrice = todayPrice
		res.todayGain = todayRange / todayPrice * 100

		//fmt.Printf("%.2f%%\n", todayRange/todayPrice*100)
	} else {
		return nil, errors.New("No enough price data")
	}
	daysAvg := stock.MA(minData)
	if len(daysAvg) > 0 {
		NDayAvg := daysAvg[len(daysAvg)-1]
		//fmt.Println(NDayAvg, todayPrice, todayPrice > NDayAvg)
		res.NDayAvg = NDayAvg
		res.overMA = todayPrice > NDayAvg
	} else {
		return nil, errors.New("No enough avg data")
	}
	return res, nil

}

//ç²åå¤è³èé¸è³
type TXXData struct {
	Buy   int64
	Sell  int64
	Total int64
}

var (
	T38DataMap map[time.Time]map[string]TXXData = make(map[time.Time]map[string]TXXData)
	T44DataMap map[time.Time]map[string]TXXData = make(map[time.Time]map[string]TXXData)
)

func getT38(date time.Time) (map[string]TXXData, error) {
	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())
	if v, ok := T38DataMap[date]; ok {
		//utils.Dbg("Reuse T38Data:%v\n", date)
		return v, nil
	}

	t38 := twse.NewTWT38U(date)
	//fmt.Println(t38.URL())
	t38Map := make(map[string]TXXData)
	if data, err := t38.Get(); err == nil {
		for _, v := range data {
			//	fmt.Printf("No: %s Buy %d Sell %d Total %d\n",
			//		v[0].No,
			//		v[0].Buy,
			//		v[0].Sell,
			//		v[0].Total)
			t38Map[v[0].No] = TXXData{v[0].Buy, v[0].Sell, v[0].Total}
		}

	} else {
		utils.Dbg("Error: %s\n", err.Error())
		if strings.Contains(err.Error(), "File No Data") {
			if err := os.Remove(utils.GetMD5FilePath(t38)); err != nil {
				return nil, err
			} else {
				if data, err = t38.Get(); err != nil {
					//if t38Map, err = getT38(date.AddDate(0,0,-1));err!=nil{
					return nil, err
					//}

				} else {
					for _, v := range data {
						//	fmt.Printf("No: %s Buy %d Sell %d Total %d\n",
						//		v[0].No,
						//		v[0].Buy,
						//		v[0].Sell,
						//		v[0].Total)
						t38Map[v[0].No] = TXXData{v[0].Buy, v[0].Sell, v[0].Total}
					}
				}
			}
		}
	}
	//fmt.Println(t38Map)
	T38DataMap[date] = t38Map
	return t38Map, nil
}
func getT44(date time.Time) (map[string]TXXData, error) {
	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())
	if v, ok := T44DataMap[date]; ok {
		//utils.Dbg("Reuse T44Data:%v\n", date)
		return v, nil
	}

	t44 := twse.NewTWT44U(date)
	//fmt.Println(t44.URL())
	t44Map := make(map[string]TXXData)
	if data, err := t44.Get(); err == nil {
		for _, v := range data {
			//	fmt.Printf("No: %s Buy %d Sell %d Total %d\n",
			//		v[0].No,
			//		v[0].Buy,
			//		v[0].Sell,
			//		v[0].Total)
			t44Map[v[0].No] = TXXData{v[0].Buy, v[0].Sell, v[0].Total}
		}

	} else {
		utils.Dbg("Error: %s\n", err.Error())
		if strings.Contains(err.Error(), "File No Data") {
			if err := os.Remove(utils.GetMD5FilePath(t44)); err != nil {
				return nil, err
			} else {
				if data, err = t44.Get(); err != nil {
					//if t44Map, err = getT44(date.AddDate(0,0,-1));err!=nil{
					return nil, err
					//}

				} else {
					for _, v := range data {
						//	fmt.Printf("No: %s Buy %d Sell %d Total %d\n",
						//		v[0].No,
						//		v[0].Buy,
						//		v[0].Sell,
						//		v[0].Total)
						t44Map[v[0].No] = TXXData{v[0].Buy, v[0].Sell, v[0].Total}
					}
				}
			}
		}
	}
	//fmt.Println(t44Map)
	T44DataMap[date] = t44Map
	return t44Map, nil
}

func getT38ByDate(stockNo string, day int) (bool, []int64) {
	var (
		overbought int
		getDay     int
	)

	data := make([]int64, day)
	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())
	RecentlyOpendtoday, _ := time.Parse(shortForm, *useDate)
	//å¾æè¿çå¤©æ¸éå§æå day å¤©ç è³æ å° å(10+day)å¤© å¦ææ²ææå° day å¤©è³æåé¯èª¤
	for i := RecentlyOpendtoday; RecentlyOpendtoday.AddDate(0, 0, -10-day).Before(i) && getDay < day; i = tradingdays.FindRecentlyOpened(i) {
		if v, err := getT38(i); err == nil {
			getDay++
			if v[stockNo].Total > 0 {
				data[overbought] = v[stockNo].Total
				overbought++
			}
		}
	}
	if getDay == day {
		return overbought == day, data
	} else {
		return false, nil
	}
}
func getT44ByDate(stockNo string, day int) (bool, []int64) {
	var (
		overbought int
		getDay     int
	)

	data := make([]int64, day)
	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())
	RecentlyOpendtoday, _ := time.Parse(shortForm, *useDate)
	for i := RecentlyOpendtoday; RecentlyOpendtoday.AddDate(0, 0, -10-day).Before(i) && getDay < day; i = tradingdays.FindRecentlyOpened(i) {
		if v, err := getT44(i); err == nil {
			getDay++
			if v[stockNo].Total > 0 {
				data[overbought] = v[stockNo].Total
				overbought++
			}
		}
	}
	if getDay == day {
		return overbought == day, data
	} else {
		return false, nil
	}
}

func getTWSE(category string, minDataNum int) error {

	RecentlyOpendtoday, _ := time.Parse(shortForm, *useDate)
	utils.Dbgln(RecentlyOpendtoday)

	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())

	t := twse.NewLists(RecentlyOpendtoday)
	tList := t.GetCategoryList(category)
	year, month, day := RecentlyOpendtoday.Date()

	csvFile, err := os.OpenFile(fmt.Sprintf("%d%02d%02d.csv", year, month, day), os.O_CREATE|os.O_RDWR, 0666)
	defer csvFile.Close()
	if err != nil {
		utils.Dbg("error: %s\n", err)
		return err
	}
	csvWriter := csv.NewWriter(csvFile)
	//	t38 ,err := getT38(RecentlyOpendtoday)
	//	if err != nil{
	//		return err
	//	}
	utils.Dbgln()
	mtssMapData, err := twse.NewTWMTSS(RecentlyOpendtoday, "ALL").GetData()
	if err != nil {
		return errors.Wrap(err, "MTSS GetData Fail.")
	}
	for _, v := range tList {
		//fmt.Printf("No:%s\n", v.No)
		stock := twse.NewTWSE(v.No, RecentlyOpendtoday)
		//checkFirstDayOfMonth(stock)
		if err := prepareStock(stock, minDataNum); err == nil {
			var output bool = true
			utils.Dbgln()
			isT38OverBought, _ := getT38ByDate(v.No, 3)
			isT44OverBought, _ := getT44ByDate(v.No, 3)
			isMTSSOverBought := mtssMapData[v.No].MT.Total > 0 && mtssMapData[v.No].SS.Total > 0
			utils.Dbgln()
			if res, err := showStock(stock, minDataNum); err == nil {
				utils.Dbgln()
				if *useCp {
					if res.todayGain >= 3.5 {
						output = true
					} else {
						output = false
					}
				}
				if *useMa {
					if !res.overMA {
						output = false
					}
				}
				if *useT38 {
					if !isT38OverBought {
						output = false
					}
				}
				if *useT44 {
					if !isT44OverBought {
						output = false
					}
				}
				if *useMtss {
					if !isMTSSOverBought {
						output = false
					}
				}
				utils.Dbgln(output)

				err = csvWriter.Write([]string{v.No,
					v.Name,
					fmt.Sprintf("%.2f", res.todayRange),
					fmt.Sprintf("%.2f", res.todayPrice),
					fmt.Sprintf("%.2f", res.todayGain),
					fmt.Sprintf("%.2f", res.NDayAvg),
					fmt.Sprintf("%t", res.overMA),
					fmt.Sprintf("%t", isT38OverBought),
					fmt.Sprintf("%t", isT44OverBought),
					fmt.Sprintf("%t", isMTSSOverBought)})
				if err != nil {
					return err
				}
				csvWriter.Flush()
				err = csvWriter.Error()
				if err != nil {
					return err
				}
				log.Printf("No:%6s Range: %6.2f Price: %6.2f Gain: %6.2f%% NDayAvg:%6.2f overMA:%t T38OverBought:%t T44OverBought:%t MTSSOverBought:%t\n",
					v.No,
					res.todayRange,
					res.todayPrice,
					res.todayGain,
					res.NDayAvg,
					res.overMA,
					isT38OverBought,
					isT44OverBought,
					isMTSSOverBought)

			}
		} else {
			fmt.Println(err)
		}
	}
	return nil

}
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	utils.Dbgln("callbackHandler")
	events, err := bot.ParseRequest(r)

	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	utils.Dbgln("ParseRequest")
	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {

			utils.Dbgln("EventTypeMessage")
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				quota, err := bot.GetMessageQuota().Do()
				if err != nil {
					log.Println("Quota err:", err)
				}
				if _, err = bot.ReplyMessage(event.ReplyToken,
					linebot.NewTextMessage(message.ID+":"+
						message.Text+
						" OK! remain message:"+
						strconv.FormatInt(quota.Value, 10))).Do(); err != nil {
					log.Print(err)
				}
			}
		}
	}
}
