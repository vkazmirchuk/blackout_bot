package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	dateFormat     = "02.01.2006"
	intervalInDays = 7                // сколько дней запрашивать
	placeFilter    = "Беляевская"     // какой нас. пункт искать
	timeout        = 30 * time.Minute // время опроса mrsk
	baseURL        = "https://mrsk-cp.ru/for_consumers/planned_emergency_outages/planned_outages_timetable/get_outages.php"
	regionNumber   = 43
	districtID     = "810F29AA-6AF6-47E9-B611-C99F7127AF52" // ID слободского района
)

var bot *tgbotapi.BotAPI
var past = map[string]bool{}
var lastMsg = "🤷‍♂️‍"
var chatID string

func main() {
	token, ok := os.LookupEnv("TOKEN")
	if !ok {
		panic("No TOKEN environment variable")
	}
	chatID, ok = os.LookupEnv("CHAT_ID")
	if !ok {
		panic("No CHAT_ID environment variable")
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(err)
	}

	go getData()   // периодически опрашиваем mrsk
	readCommands() // читаем команды от пользователей в чате
}

func getData() {
	currentDate := time.Now().Format(dateFormat)
	endDate := time.Now().AddDate(0, 0, intervalInDays).Format(dateFormat)

	reqUrl := fmt.Sprintf("%s?region=%d&district=%s&begin_date=%s&end_date=%s", baseURL, regionNumber, districtID, currentDate, endDate)

	log.Println("GET:", reqUrl)

	resp, err := http.Get(reqUrl)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	log.Println("RESP:", resp.ContentLength, "bytes")

	html, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(err)
	}

	parseHTML(html)

	time.Sleep(timeout)

	getData()
}

// парсинг html кода и поиск необходимой информации
func parseHTML(html *goquery.Document) {
	html.Find("tbody tr").Each(func(i int, tr *goquery.Selection) {
		place := tr.Find("#col-place")
		bdate := tr.Find("#col-bdate")
		edate := tr.Find("#col-edate")

		// ищем нужный нас. пункт
		if strings.Contains(place.Text(), placeFilter) {
			msg := fmt.Sprintf("⚠️ с %s %s до %s %s ⚠️", bdate.Text(), bdate.Next().Text(), edate.Text(), edate.Next().Text())

			if past[msg] {
				return
			}

			if err := sendMessageToChat(msg); err != nil {
				log.Println(err)
				return
			}
			past[msg] = true
			lastMsg = msg
		}
	})
}

func readCommands() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		panic(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// если прислали запрос на прошлые показания
		if update.Message.Text == fmt.Sprintf("/show@%s", bot.Self.UserName) {

			log.Printf("User %s requested data", update.Message.From.String())

			if err := sendMessageToChat(lastMsg); err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func sendMessageToChat(msg string) error {
	_, err := bot.Send(tgbotapi.NewMessageToChannel(chatID, msg))
	if err != nil {
		return err
	}
	log.Println("Sent to chat:", msg)
	return err
}
