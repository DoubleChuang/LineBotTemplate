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

var replyHelp = `
DD [YYYYMMDD] [命令] [命令參數]
ex.
	DD 20190703 股票 2330
`
var defaultB = false
var useMtss = &defaultB
var useT38 = &defaultB
var useT44 = &defaultB
var useMa = &defaultB
var useCp = &defaultB
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
func prepareStock(stock *twse.Data, mindata int) error {

	if _, err := (stock).Get(); err != nil {
		return err
	}

	if (stock).Len() < mindata {
		start := (stock).Len()
		for {
			(stock).PlusData()
			if (stock).Len() > mindata {
				break
			}
			if (stock).Len() == start {
				break
			}
			start = (stock).Len()
		}
		if (stock).Len() < mindata {
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

func checkFilter(filter ...string) (cp /*cost price*/, ma, fi /*foreign investment*/, it /*nvestment Trust*/, mt /*margin trading*/ bool) {
	for _, v := range filter {
		if v == "股價" {
			cp = true
		} else if v == "均線" {
			ma = true
		} else if v == "外資" {
			fi = true
		} else if v == "投信" {
			it = true
		} else if v == "資券" {
			mt = true
		}
	}
	return
}

func getTWSEByFilter(date time.Time, stockNo string, t38 **twse.TWT38U, t44 **twse.TWT44U, mtss **twse.TWMTSS, filter ...string) (string, error) {
	var ret string
	var found bool
	var output = true

	var cp, ma, fi /*foreign investment*/, it /*nvestment Trust*/, mt /*margin trading*/ bool
	cp, ma, fi, it, mt = checkFilter(filter...)
	t := twse.NewLists(date)
	tList := t.GetCategoryList("ALLBUT0999")

	var pStock *twse.Data
	for _, v := range tList {
		if v.No == stockNo {
			pStock = twse.NewTWSE(stockNo, date)
			found = true
			break
		}
	}
	if !found {
		return fmt.Sprintf("%s沒有%s此股票", date.Format(shortForm), stockNo), errors.Errorf("%s沒有%s此股票", date.Format(shortForm), stockNo)
	}
	//}

	mtssMapData, err := (*mtss).SetDate(date).GetData()
	if err != nil {
		return fmt.Sprintf("融資融券資料錯誤"), errors.Errorf("融資融券資料錯誤")
	}
	if err := prepareStock(pStock, 20); err == nil {
		var d time.Time
		for _, d = range pStock.GetDateList() {
			if d == date {
				break
			}
		}

		isT38OverBought, _ := (*t38).IsOverBoughtDates(stockNo, 3)
		isT44OverBought, _ := (*t44).IsOverBoughtDates(stockNo, 3)
		if s, err := showStock(pStock, 20); err == nil {
			if cp {
				if s.todayGain >= 3.5 {
					output = true
				} else {
					output = false
				}
			}
			if ma {
				if !s.overMA {
					output = false
				}
			}
			if fi {
				if !isT38OverBought {
					output = false
				}
			}
			if it {
				if !isT44OverBought {
					output = false
				}
			}
			if mt {
				if !(mtssMapData[stockNo].MT.Total > 0 && mtssMapData[stockNo].SS.Total > 0) {
					output = false
				}
			}
			if output {
				ret = fmt.Sprintf(
					"%s：%s", pStock.No, pStock.Name)
			}
		}
	} else {
		ret = fmt.Sprintf("%s 資料錯誤", pStock.No)
	}

	return ret, nil

}

func getOneTWSE(date time.Time, stockNo string, t38 **twse.TWT38U, t44 **twse.TWT44U, mtss **twse.TWMTSS) (string, error) {
	var ret string

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
		return fmt.Sprintf("%s沒有%s此股票", date.Format(shortForm), stockNo), errors.Errorf("%s沒有%s此股票", date.Format(shortForm), stockNo)
	}
	//}

	mtssMapData, err := (*mtss).SetDate(date).GetData()
	if err != nil {
		return fmt.Sprintf("融資融券資料錯誤"), errors.Errorf("融資融券資料錯誤")
	}
	if err := prepareStock(pStock, 20); err == nil {
		var d time.Time
		for _, d = range pStock.GetDateList() {
			if d == date {
				break
			}
		}

		isT38OverBought, x := (*t38).IsOverBoughtDates(stockNo, 3)
		isT44OverBought, y := (*t44).IsOverBoughtDates(stockNo, 3)
		if s, err := showStock(pStock, 20); err == nil {
			ret = fmt.Sprintf(
				"\n%4s：%6.2f\n"+
					"%4s：%6.2f\n"+
					"%4s：%6.2f%%\n"+
					"%4s：%6.2f\n"+
					"%4s：%t\n"+
					"%4s：%t %d\n"+
					"%4s：%t %d\n"+
					"%4s：%t %d\n"+
					"%4s：%t %d\n",
				"漲跌價", s.todayRange,
				"成交價", s.todayPrice,
				"漲跌幅", s.todayGain,
				"月均價", s.NDayAvg,
				"破月均", s.overMA,
				"外資增", isT38OverBought, x[0]/1000,
				"投信增", isT44OverBought, y[0]/1000,
				"融資增", mtssMapData[stockNo].MT.Total > 0, mtssMapData[stockNo].MT.Total,
				"融券增", mtssMapData[stockNo].SS.Total > 0, mtssMapData[stockNo].SS.Total,
			)
		}
	} else {
		ret = fmt.Sprintf("資料錯誤")
	}

	return ret, nil

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
		if err := prepareStock(stock, minDataNum); err == nil {

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
				var replyMsg = "股票列表\n=========\n"

				quota, err := bot.GetMessageQuota().Do()
				if err != nil {
					log.Println("Quota err:", err)
				}
				if quota.Value != 500 {
					replyMsg = fmt.Sprintf("剩餘訊息用量：%d\n%s", quota.Value, replyMsg)
				}
				var msgList []linebot.SendingMessage
				if reqTime, reqCmd, remainList, err := parserMsg(message.Text); err == nil {
					if reqCmd == "股票" && len(remainList) != 0 {
						for _, v := range remainList {
							var ret string
							if ret, err = getOneTWSE(reqTime, v, &T38U, &T44U, &MTSS); err != nil {
								replyMsg = fmt.Sprintf("股票[%s] 發生錯誤:\n%s", v, err.Error())
							} else {
								replyMsg = fmt.Sprintf("股票[%s]:\n%s", v, ret)
							}
							msg := linebot.NewTextMessage(replyMsg)
							msgList = append(msgList, msg)

						}
					} else if reqCmd == "股票分析" {
						t := twse.NewLists(reqTime)
						tList := t.GetCategoryList("ALLBUT0999")

						for _, stockInfo := range tList {
							var ret string
							if ret, err = getTWSEByFilter(reqTime, stockInfo.No, &T38U, &T44U, &MTSS, remainList...); err != nil {
								replyMsg = fmt.Sprintf("股票[%s] 發生錯誤:\n%s", stockInfo.No, err.Error())
							} else {
								if len(ret) > 0 {
									if (len(replyMsg) + len(ret)) < 2000 {
										replyMsg = fmt.Sprintf("%s\n%s", replyMsg, ret)
									} else {
										msg := linebot.NewTextMessage(replyMsg)
										msgList = append(msgList, msg)
										replyMsg = fmt.Sprintf("%s", ret)
									}
								}
							}

						}
						msg := linebot.NewTextMessage(replyMsg)
						msgList = append(msgList, msg)

					} else {
						replyMsg = fmt.Sprintf("不支援此命令：%s\n%s\n", reqCmd, replyHelp)
						msg := linebot.NewTextMessage(replyMsg)
						msgList = append(msgList, msg)
					}

					if _, err = bot.ReplyMessage(event.ReplyToken,
						msgList...).Do(); err != nil {
						log.Print(err)
					}
				} else {
					if err != nil && !strings.Contains(err.Error(), "不是命令") {
						replyMsg = fmt.Sprintf("%s\n%s", err.Error(), replyHelp)
						if _, err = bot.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage(replyMsg)).Do(); err != nil {
							log.Print(err)
						}
					} else if err == nil {
						if _, err = bot.ReplyMessage(event.ReplyToken,
							linebot.NewTextMessage("你忘了輸入參數")).Do(); err != nil {
							log.Print(err)
						}
					}
				}

			}
		}
	}
}
