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
	"github.com/goodsign/monday"
)

const (
	dateFormat     = "02.01.2006"
	dateTimeFormat = "02.01.2006 15:04"
	intervalInDays = 7                // сколько дней запрашивать
	placeFilter    = "Беляевская"     // какой нас. пункт искать
	timeout        = 30 * time.Minute // время опроса mrsk
	baseURL        = "https://mrsk-cp.ru/for_consumers/planned_emergency_outages/planned_outages_timetable/get_outages.php"
	regionNumber   = 43
	districtID     = "810F29AA-6AF6-47E9-B611-C99F7127AF52" // ID слободского района
)

var bot *tgbotapi.BotAPI
var past = make([]string, 0)
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

			startDateTime, err := time.Parse(dateTimeFormat, fmt.Sprintf("%s %s", bdate.Text(), bdate.Next().Text()))
			if err != nil {
				log.Println(err)
				return
			}

			endDateTime, err := time.Parse(dateTimeFormat, fmt.Sprintf("%s %s", edate.Text(), edate.Next().Text()))
			if err != nil {
				log.Println(err)
				return
			}

			msgStart := monday.Format(startDateTime, "Mon 02 Jan 15:04", monday.LocaleRuRU)
			msgEnd := monday.Format(endDateTime, "Mon 02 Jan 15:04", monday.LocaleRuRU)
			duration := endDateTime.Sub(startDateTime)

			if startDateTime.Day() == endDateTime.Day() {
				msgEnd = endDateTime.Format("15:04")
			}

			msg := fmt.Sprintf("⚠️ %s до %s (отключат на %s) ⚠️", msgStart, msgEnd, duration)

			for _, pm := range past {
				if pm == msg {
					return
				}
			}

			msgTg, err := sendMessageToChat(msg)
			if err != nil {
				log.Println(err)
				return
			}

			if err := PinnedMessage(msgTg); err != nil {
				log.Println(err)
				return
			}

			past = append(past, msg)
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

			if _, err := sendMessageToChat(lastMsg); err != nil {
				log.Println(err)
				continue
			}
		}
	}
}

func sendMessageToChat(msg string) (msgTg tgbotapi.Message, err error) {
	msgTg, err = bot.Send(tgbotapi.NewMessageToChannel(chatID, msg))
	if err != nil {
		return msgTg, err
	}

	log.Println("Sent to chat and pinned:", msg)

	return msgTg, err
}

func PinnedMessage(msg tgbotapi.Message) error {
	_, err := bot.PinChatMessage(tgbotapi.PinChatMessageConfig{
		ChatID:              msg.Chat.ID,
		MessageID:           msg.MessageID,
		DisableNotification: false,
	})

	return err
}
