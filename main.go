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
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DoubleChuang/gogrs/twse"
	"github.com/DoubleChuang/gogrs/utils"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/pkg/errors"
	"github.com/toomore/gogrs/tradingdays"
)

const shortForm = "20060102"

var replyHelp string = `
DD [YYYYMMDD] [命令] [命令參數]
ex.
	DD 20190703 股票 2330
`
var defaultB bool = false
var useMtss *bool = &defaultB
var useT38 *bool = &defaultB
var useT44 *bool = &defaultB
var useMa *bool = &defaultB
var useCp *bool = &defaultB
var defaultDate string = "20190705"
var useDate *string = &defaultDate
var bot *linebot.Client

//ç²åå¤è³èé¸è³
type TXXData struct {
	Buy   int64
	Sell  int64
	Total int64
}
type resData struct {
	todayRange float64
	todayPrice float64
	todayGain  float64
	NDayAvg    float64
	overMA     bool
}

var (
	T38DataMap  map[time.Time]map[string]TXXData   = make(map[time.Time]map[string]TXXData)
	T44DataMap  map[time.Time]map[string]TXXData   = make(map[time.Time]map[string]TXXData)
	TWSEDataMap map[time.Time]map[string]twse.Data = make(map[time.Time]map[string]twse.Data)
)

type ARGV int64

const (
	DD ARGV = iota
	TIME
	CMD
	REMAIN
)

func main() {
	var err error
	utils.Dbg("T38U:%p T44U:%p MTSS:%p\n", T38U, T44U, MTSS)
	go initStock(tradingdays.FindRecentlyOpened(time.Now()), &T38U, &T44U, &MTSS)
	utils.Dbg("T38U:%p T44U:%p MTSS:%p\n", T38U, T44U, MTSS)
	//go getTWSE("20190705", 20, T38U, T44U, MTSS))
	utils.Dbgln(utils.GetOSRamdiskPath(""))

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

var T38U *twse.TWT38U
var T44U *twse.TWT44U
var MTSS *twse.TWMTSS

func initStock(date time.Time, t38 **twse.TWT38U, t44 **twse.TWT44U, mtss **twse.TWMTSS) error {
	var err error

	if *t38 == nil {
		*t38 = twse.NewTWT38U(date)
	}
	if *t44 == nil {
		*t44 = twse.NewTWT44U(date)
	}
	if *mtss == nil {
		*mtss = twse.NewTWMTSS(date, "ALL")
	}

	if _, err = (*t38).GetData(); err != nil {
		errors.Wrap(err, "Get T38U Data Fail")
	}
	if _, err = (*t44).GetData(); err != nil {
		errors.Wrap(err, "Get T44U Data Fail")
	}
	if _, err = (*mtss).GetData(); err != nil {
		errors.Wrap(err, "Get MTSS Data Fail")
	}
	return err
	/*if err := getTWSE(date, "ALLBUT0999", 20, T38U, T44U, MTSS); err != nil {
		utils.Dbgln(err)
	}*/
}
func parserMsg(msg string) (reqTime time.Time, reqCmd string, remainList []string, err error) {
	reqCmd = "股票"
	remainList = make([]string, 0)

	reqString := strings.TrimSpace(msg)
	reqList := strings.Split(reqString, " ")

	//用來計算跑到哪個
	var c ARGV

	for i := 0; i < len(reqList); i++ {
		v := reqList[i]
		//跳過空白
		if len(v) == 0 {
			continue
		}
		switch c {
		case DD:
			if v != "DD" {
				err = errors.New("不是命令")
				break
			}
			c++
		case TIME:
			if reqTime, err = time.Parse(shortForm, v); err != nil {
				reqTime = tradingdays.FindRecentlyOpened(time.Now())
				err = nil

				i--
			}
			c++
		case CMD:
			if strings.Contains(v, "股票") ||
				strings.Contains(v, "股票分析") {
				reqCmd = v
			} else {

				err = errors.New("輸入格式錯誤")
				break
			}
			c++
		case REMAIN:
			remainList = append(remainList, v)
		default:
		}
	}
	utils.Dbgln(reqTime)
	utils.Dbgln(reqCmd)
	utils.Dbgln(remainList)
	utils.Dbgln(err)
	return reqTime, reqCmd, remainList, err
}
func prepareStock(stock **twse.Data, mindata int) error {

	if _, err := (*stock).Get(); err != nil {
		return err
	}

	if (*stock).Len() < mindata {
		start := (*stock).Len()
		for {
			(*stock).PlusData()
			if (*stock).Len() > mindata {
				break
			}
			if (*stock).Len() == start {
				break
			}
			start = (*stock).Len()
		}
		if (*stock).Len() < mindata {
			return errors.New("Can't prepare enough data, please check file has data or remove cache file")
		}
	}
	return nil
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

func getOneTWSE(date time.Time, stockNo string, t38 **twse.TWT38U, t44 **twse.TWT44U, mtss **twse.TWMTSS) string {
	var ret string
	//stock, ok := TWSEDataMap[date][stockNo]
	//pStock := &stock
	//if !ok {
	t := twse.NewLists(date)
	tList := t.GetCategoryList("ALLBUT0999")
	found := false
	var pStock *twse.Data
	for _, v := range tList {
		if v.No == stockNo {
			pStock = twse.NewTWSE(stockNo, date)
			found = true
			break
		}
	}
	if !found {
		return fmt.Sprintf("%s沒有%s此股票", date.Format(shortForm), stockNo)
	}
	//}
	utils.Dbgln(pStock.Date)
	//twse.NewTWMTSS(date, "ALL")
	mtssMapData, err := (*mtss).SetDate(date).GetData()
	if err != nil {
		return fmt.Sprintf("融資融券資料錯誤")
	}
	if err := prepareStock(&pStock, 20); err == nil {
		isT38OverBought, _ := (*t38).IsOverBoughtDates(stockNo, 3)
		isT44OverBought, _ := (*t44).IsOverBoughtDates(stockNo, 3)
		if s, err := showStock(pStock, 20); err == nil {
			ret = fmt.Sprintf("漲跌: %.2f\n成交價: %.2f\n漲跌幅: %.2f%%\n20MA:%.2f\n突破MA:%t\n外資增：%t\n投信增:%t\n融資增：%t\n融券增：%t\n=========\n",
				s.todayRange,
				s.todayPrice,
				s.todayGain,
				s.NDayAvg,
				s.overMA,
				isT38OverBought,
				isT44OverBought,
				mtssMapData[stockNo].MT.Total > 0,
				mtssMapData[stockNo].SS.Total > 0,
			)
		}
	} else {
		ret = fmt.Sprintf("資料錯誤")
	}

	if _, exist := TWSEDataMap[date]; exist {
		TWSEDataMap[date][stockNo] = *pStock
	} else {
		twseData := make(map[string]twse.Data)
		twseData[stockNo] = *pStock
		TWSEDataMap[date] = twseData
	}

	return ret

}

func getTWSE(useDate string, minDataNum int, t38 *twse.TWT38U, t44 *twse.TWT44U, mtss *twse.TWMTSS) error {

	if err := utils.RecoveryStockBackup(useDate); err != nil {
		utils.Dbgln(err)
	}

	RecentlyOpendtoday, _ := time.Parse(shortForm, useDate)
	utils.Dbgln(RecentlyOpendtoday)

	//RecentlyOpendtoday := tradingdays.FindRecentlyOpened(time.Now())

	t := twse.NewLists(RecentlyOpendtoday)
	tList := t.GetCategoryList("ALLBUT0999")
	MTSS = twse.NewTWMTSS(RecentlyOpendtoday, "ALL")
	mtssMapData, err := MTSS.GetData()
	if err != nil {
		return errors.Wrap(err, "MTSS GetData Fail.")
	}
	tmpStock := make(map[string]twse.Data, len(tList))
	for _, v := range tList {
		//fmt.Printf("No:%s\n", v.No)
		stock := twse.NewTWSE(v.No, RecentlyOpendtoday)
		//checkFirstDayOfMonth(stock)
		if err := prepareStock(&stock, minDataNum); err == nil {

			tmpStock[v.No] = *stock
			var output bool = true

			isT38OverBought, _ := t38.IsOverBoughtDates(v.No, 3)
			isT44OverBought, _ := t44.IsOverBoughtDates(v.No, 3)
			isMTSSOverBought := mtssMapData[v.No].MT.Total > 0 && mtssMapData[v.No].SS.Total > 0

			if res, err := showStock(stock, minDataNum); err == nil {
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
				if output {
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

			}
		} else {
			fmt.Println(err)
		}
	}
	TWSEDataMap[RecentlyOpendtoday] = tmpStock
	return nil
}
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	events, err := bot.ParseRequest(r)

	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400)
		} else {
			w.WriteHeader(500)
		}
		return
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {

			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				var replyMsg string = "股票列表\n=========\n"

				quota, err := bot.GetMessageQuota().Do()
				if err != nil {
					log.Println("Quota err:", err)
				}
				if quota.Value != 500 {
					replyMsg = fmt.Sprintf("剩餘訊息用量：%d\n%s", quota.Value, replyMsg)
				}

				if reqTime, reqCmd, remainList, err := parserMsg(message.Text); err == nil {
					dateDataMap, _ := TWSEDataMap[reqTime]
					//if ; ok {
					utils.Dbgln()
					if reqCmd == "股票" {
						utils.Dbgln()
						for _, v := range remainList {
							/*var ret string
							stockData, ok := dateDataMap[v]
							if ok {
								if res, err := showStock(&stockData, 20); err == nil {
									ret = fmt.Sprintf("漲跌: %.2f\n成交價: %.2f\n漲跌幅: %.2f%%\n20MA:%.2f\n突破MA:%t\n=========\n",
										res.todayRange,
										res.todayPrice,
										res.todayGain,
										res.NDayAvg,
										res.overMA,
									)
								}

							} else {
								name = "搜尋不到"
							}*/
							var ret string
							ret = getOneTWSE(reqTime, v, &T38U, &T44U, &MTSS)
							replyMsg = fmt.Sprintf("%s\n股票[%s]:\n%s", replyMsg, v, ret)
						}
					} else if reqCmd == "股票分析" {
						utils.Dbgln()
						for _, v := range dateDataMap {
							if res, err := showStock(&v, 20); err == nil {
								//mtssMapData, err := twse.NewTWMTSS(reqTime, "ALL").GetData()
								if err != nil {
									//return errors.Wrap(err, "MTSS GetData Fail.")
								}
								//isT38OverBought, _ := getT38ByDate(v.No, 3)
								//isT44OverBought, _ := getT44ByDate(v.No, 3)
								//isMTSSOverBought := mtssMapData[v.No].MT.Total > 0 && mtssMapData[v.No].SS.Total > 0
								isGainOver3 := res.todayGain >= 3.5
								isPriceOverMA := res.overMA
								if /*isT38OverBought && isT44OverBought && isMTSSOverBought &&*/
								isGainOver3 && isPriceOverMA {
									replyMsg = fmt.Sprintf("%sNo: %6s\n漲跌: %.2f\n成交價: %.2f\n漲跌幅: %.2f%%\n20MA:%.2f\n突破MA:%t\n=========\n",
										replyMsg,
										v.No,
										res.todayRange,
										res.todayPrice,
										res.todayGain,
										res.NDayAvg,
										res.overMA,
									)
								}
							} else {
								utils.Dbgln(err)
							}
						}

					} else {
						replyMsg = fmt.Sprintf("不支援此命令：%s\n%s\n", reqCmd, replyHelp)
					}
					/*} else {
						replyMsg = fmt.Sprintf("%s這個時間目前沒有資料", reqTime.Format("2006-01-02"))
					}*/
					if _, err = bot.ReplyMessage(event.ReplyToken,
						linebot.NewTextMessage(replyMsg)).Do(); err != nil {
						log.Print(err)
					}
				} else {
					if !strings.Contains(err.Error(), "不是命令") {
						replyMsg = fmt.Sprintf("%s\n%s", err.Error(), replyHelp)
						if _, err = bot.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage(replyMsg)).Do(); err != nil {
							log.Print(err)
						}
					}
				}

			}
		}
	}
}
