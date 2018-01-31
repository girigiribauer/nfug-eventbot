勉強会の slackbot です、さくっと作っただけなので汎用性はないです

* cron.yaml に書いてある情報を元に、1時間ごとに GoogleAppEngine が slackbot.go を動かす
* slackbot.go は connpass API <https://connpass.com/about/api/> からイベントを取得し、条件にマッチした場合のみ slack API <https://api.slack.com/incoming-webhooks> にリクエストを投げる
