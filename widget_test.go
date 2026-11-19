package telebot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWidgetQueryUpdate(t *testing.T) {
	b, err := NewBot(Settings{Synchronous: true, Offline: true})
	require.NoError(t, err)

	var handled bool
	b.Handle(OnWidgetQuery, func(c Context) error {
		assert.Equal(t, "refresh", c.Data())
		assert.Equal(t, int64(1), c.Sender().ID)
		assert.Equal(t, int64(2), c.Chat().ID)
		assert.Equal(t, "2", c.Recipient().Recipient())
		assert.Equal(t, "query-id", c.WidgetQuery().ID)

		handled = true
		return nil
	})

	b.ProcessUpdate(Update{WidgetQuery: &WidgetQuery{
		ID:     "query-id",
		Sender: &User{ID: 1},
		Chat:   &Chat{ID: 2},
		Data:   "refresh",
	}})

	assert.True(t, handled)
}

func TestWidgetJSON(t *testing.T) {
	data := []byte(`{
		"widget_query": {
			"id": "query-id",
			"from": {"id": 1, "first_name": "Sender"},
			"chat": {"id": 2, "type": "private"},
			"message_id": 3,
			"chat_instance": "instance",
			"type": {"type": "data"},
			"data": "refresh"
		}
	}`)

	var update Update
	require.NoError(t, json.Unmarshal(data, &update))
	require.NotNil(t, update.WidgetQuery)

	query := update.WidgetQuery
	assert.Equal(t, int64(1), query.Sender.ID)
	assert.Equal(t, "data", query.Type.Type)
	assert.Equal(t, "refresh", query.Data)

	msgID, chatID := query.MessageSig()
	assert.Equal(t, "3", msgID)
	assert.Equal(t, int64(2), chatID)

	var msg Message
	require.NoError(t, json.Unmarshal([]byte(`{
		"message_id": 3,
		"widget": {"html": "<b>hi</b>", "height": 120, "chat_data": "state"}
	}`), &msg))
	require.NotNil(t, msg.Widget)
	assert.Equal(t, "<b>hi</b>", msg.Widget.HTML)
	assert.Equal(t, 120, msg.Widget.Height)
	assert.Equal(t, "state", msg.Widget.ChatData)
}

func TestWidgetQueryInlineMessageSig(t *testing.T) {
	msgID, chatID := (&WidgetQuery{InlineMessageID: "inline-id"}).MessageSig()
	assert.Equal(t, "inline-id", msgID)
	assert.Zero(t, chatID)
}

// widgetAPI serves a single Bot API method and records the parameters it got.
func widgetAPI(t *testing.T, method, response string) (*Bot, map[string]interface{}) {
	body := make(map[string]interface{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/bottoken/"+method, r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)

	b, err := NewBot(Settings{
		Offline: true,
		Token:   "token",
		URL:     srv.URL,
		Client:  srv.Client(),
	})
	require.NoError(t, err)
	return b, body
}

func TestSendWidget(t *testing.T) {
	b, body := widgetAPI(t, "sendWidget",
		`{"ok":true,"result":{"message_id":3,"widget":{"html":"<b>hi</b>"}}}`)

	markup := &ReplyMarkup{}
	markup.Inline(markup.Row(markup.Data("Refresh", "refresh")))

	msg, err := b.Send(&Chat{ID: 2}, &Widget{
		HTML:         "<b>hi</b>",
		Height:       120,
		ChatData:     "state",
		Caption:      "caption",
		AllowQueries: true,
	}, markup, ModeHTML)
	require.NoError(t, err)

	assert.Equal(t, 3, msg.ID)
	assert.Equal(t, "<b>hi</b>", msg.Widget.HTML)

	assert.Equal(t, "2", body["chat_id"])
	assert.Equal(t, "<b>hi</b>", body["html"])
	assert.Equal(t, "120", body["height"])
	assert.Equal(t, "state", body["chat_data"])
	assert.Equal(t, "caption", body["caption"])
	assert.Equal(t, ModeHTML, body["parse_mode"])
	assert.Equal(t, "true", body["allow_queries"])
	assert.Contains(t, body["reply_markup"], "Refresh")
	assert.NotContains(t, body, "width")
	assert.NotContains(t, body, "public_data")
}

func TestEditWidgetChatData(t *testing.T) {
	b, body := widgetAPI(t, "editMessageWidgetData", `{"ok":true,"result":true}`)

	require.NoError(t, b.EditWidgetChatData(&Message{ID: 3, Chat: &Chat{ID: 2}}, ""))

	assert.Equal(t, "2", body["chat_id"])
	assert.Equal(t, "3", body["message_id"])
	assert.Equal(t, "", body["chat_data"])
	assert.Contains(t, body, "chat_data")
	assert.NotContains(t, body, "public_data")
}

func TestAnswerWidget(t *testing.T) {
	b, body := widgetAPI(t, "answerWidgetQuery", `{"ok":true,"result":true}`)

	require.NoError(t, b.AnswerWidget(&WidgetQuery{ID: "query-id"}, "ok"))

	assert.Equal(t, "query-id", body["widget_query_id"])
	assert.Equal(t, "ok", body["data"])
}

func TestContextAnswerWidget(t *testing.T) {
	c := &nativeContext{}
	assert.EqualError(t, c.AnswerWidget("ok"), "telebot: context widget query is nil")
}

func TestAllowedUpdatesIncludesWidgetQuery(t *testing.T) {
	assert.Contains(t, AllowedUpdates, "widget_query")
}
