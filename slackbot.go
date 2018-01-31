package slackbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

const (
	location            = "Asia/Tokyo"
	connpassURL         = "https://connpass.com/api/v1/event/"
	slackURL            = "https://nfug.slack.com/"
	connpassGroupID     = "964,4986" // 964: html5nagoya, 4986: nfug
	regularHour         = 19
	textTwoWeeksBefore1 = "2週間前になりました。参加者はそれなりに多いようです。やったね！"
	textTwoWeeksBefore2 = "2週間前になりました。参加者が少し少ないようです。みんなで宣伝しましょう！"
	textOneWeekBefore   = "1週間前になりました。次回の会場が決まっていない場合は検討しましょう。"
	textTwoDaysBefore   = "2日前です。当日参加できないことが分かっている方は、前日までにキャンセルしましょう。"
	textStart           = "イベントスタートです！\nTwitter のハッシュタグ #nfug (https://twitter.com/search?q=%23nfug) もご活用ください！"
	textNextDay         = "昨日のイベントお疲れさまでした。次のイベントが立っていなければ用意しましょう！"
)

var (
	slackbotURL = os.Getenv("SLACKBOT_URL")
)

// EventResults JSON Data
// ref: https://connpass.com/about/api/
type EventResults struct {
	Events []struct {
		Title     string    `json:"title"`
		URL       string    `json:"event_url"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
		Place     string    `json:"place"`
		Limit     int       `json:"limit"`
		Accepted  int       `json:"accepted"`
	} `json:"events"`
}

func parseEventResults(rawText []byte) EventResults {
	var eventResults EventResults

	if err := json.Unmarshal(rawText, &eventResults); err != nil {
		return EventResults{}
	}

	return eventResults
}

func getConnpassEvents(w http.ResponseWriter, r *http.Request) EventResults {
	// ref: https://cloud.google.com/appengine/docs/standard/go/issue-requests
	ctx := appengine.NewContext(r)
	client := urlfetch.Client(ctx)

	resp, err := client.Get(fmt.Sprintf("%s?count=5&order=2&series_id=%s", connpassURL, connpassGroupID))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return EventResults{}
	}

	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	return parseEventResults(body)
}

func isStartTime(startTime time.Time) bool {
	now := time.Now()
	afterOneHour := startTime.Add(time.Hour)

	return startTime.Before(now) && afterOneHour.After(now)
}

func isRegularTime() bool {
	now := time.Now()
	regularTime := time.Date(now.Year(), now.Month(), now.Day(), regularHour, 0, 0, 0, time.Local)
	afterOneHour := regularTime.Add(time.Hour)

	return regularTime.Before(now) && afterOneHour.After(now)
}

func isDaysBefore(target time.Time, days int) bool {
	now := time.Now()

	return target.YearDay()-days == now.YearDay()
}

func isQuietEvent(accepted, limit int) bool {
	return float64(accepted)/float64(limit) <= 0.5
}

func slackbot(w http.ResponseWriter, r *http.Request, url, channel, body string) {
	buffer, _ := json.Marshal(map[string]interface{}{
		"channnel": channel,
		"text":     body,
	})

	// ref: https://cloud.google.com/appengine/docs/standard/go/issue-requests
	ctx := appengine.NewContext(r)
	client := urlfetch.Client(ctx)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(buffer))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	fmt.Println(resp)
}

func handle(w http.ResponseWriter, r *http.Request) {
	eventResults := getConnpassEvents(w, r)

	if len(eventResults.Events) == 0 {
		fmt.Fprintln(w, "no events")
		return
	}

	for _, event := range eventResults.Events {
		// notification: 2 weeks ago
		if isRegularTime() && isDaysBefore(event.StartedAt, 14) {
			message := ""
			if isQuietEvent(event.Accepted, event.Limit) {
				message = textTwoWeeksBefore2
			} else {
				message = textTwoWeeksBefore1
			}

			bottext := fmt.Sprintf("『%s』%s <%s>\n", event.Title, message, event.URL)
			slackbot(w, r, slackbotURL, "#general", bottext)
		}

		// notification: 1 week ago
		if isRegularTime() && isDaysBefore(event.StartedAt, 7) {
			bottext := fmt.Sprintf("『%s』%s\n", event.Title, textOneWeekBefore)
			slackbot(w, r, slackbotURL, "#manage", bottext)
		}

		// notification: 2 days ago
		if isRegularTime() && isDaysBefore(event.StartedAt, 2) {
			bottext := fmt.Sprintf("『%s』%s <%s>\n", event.Title, textTwoDaysBefore, event.URL)
			slackbot(w, r, slackbotURL, "#general", bottext)
		}

		// notification: event start
		if isStartTime(event.StartedAt) {
			bottext := fmt.Sprintf("『%s』%s\n", event.Title, textStart)
			slackbot(w, r, slackbotURL, "#general", bottext)
		}

		// notification: event next day
		if isRegularTime() && isDaysBefore(event.StartedAt, -1) {
			bottext := fmt.Sprintf("『%s』%s\n", event.Title, textNextDay)
			slackbot(w, r, slackbotURL, "#general", bottext)
		}
	}

	fmt.Fprintln(w, slackURL)
}

func init() {
	loc, err := time.LoadLocation(location)
	if err != nil {
		loc = time.FixedZone(location, 9*60*60)
	}
	time.Local = loc

	http.HandleFunc("/", handle)
}
