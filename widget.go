package telebot

import "strconv"

// Widget describes an HTML widget (Bot API 10.4): a self-contained HTML
// document shown inside a message. Its code is fixed once sent; the bot
// changes what it displays through its data.
//
// Widget implements Sendable:
//
//	b.Send(to, &telebot.Widget{HTML: page, ChatData: state, AllowQueries: true})
type Widget struct {
	// (Optional) Unique identifier of a shared widget.
	ID string `json:"id,omitempty"`

	// (Optional) Width in logical pixels; the whole bubble width if omitted.
	Width int `json:"width,omitempty"`

	// (Optional) Height in logical pixels; not fixed if omitted.
	Height int `json:"height,omitempty"`

	// HTML code of the widget, up to 32768 bytes and up to 4096 bytes
	// when compressed by zlib with the maximum compression level.
	HTML string `json:"html"`

	// (Optional) Data available wherever the widget is shown, up to 1024 bytes.
	PublicData string `json:"public_data,omitempty"`

	// (Optional) Data specific to the message, up to 1024 bytes.
	ChatData string `json:"chat_data,omitempty"`

	// (Optional) Caption to send the widget with.
	Caption string `json:"-"`

	// (Optional) Allows the sent widget to send queries to the bot.
	AllowQueries bool `json:"-"`
}

// Send delivers the widget through bot b to recipient via the sendWidget
// method, satisfying the Sendable interface.
func (w *Widget) Send(b *Bot, to Recipient, opt *SendOptions) (*Message, error) {
	params := map[string]string{
		"chat_id": to.Recipient(),
		"html":    w.HTML,
	}
	if w.Width != 0 {
		params["width"] = strconv.Itoa(w.Width)
	}
	if w.Height != 0 {
		params["height"] = strconv.Itoa(w.Height)
	}
	if w.PublicData != "" {
		params["public_data"] = w.PublicData
	}
	if w.ChatData != "" {
		params["chat_data"] = w.ChatData
	}
	if w.Caption != "" {
		params["caption"] = w.Caption
	}
	if w.AllowQueries {
		params["allow_queries"] = "true"
	}
	b.embedSendOptions(params, opt)

	data, err := b.Raw("sendWidget", params)
	if err != nil {
		return nil, err
	}

	return extractMessage(data)
}

// WidgetQuery is an incoming query from a widget in a message sent
// by the bot. It must be answered within 15 seconds.
type WidgetQuery struct {
	// Unique identifier for this query.
	ID string `json:"id"`

	// Sender of the query.
	Sender *User `json:"from"`

	// (Optional) Chat with the message containing the widget.
	Chat *Chat `json:"chat,omitempty"`

	// (Optional) Identifier of the message containing the widget.
	MessageID int `json:"message_id,omitempty"`

	// (Optional) Identifier of the inline message containing the widget.
	InlineMessageID string `json:"inline_message_id,omitempty"`

	// Global identifier of the chat the widget's message was sent to.
	ChatInstance string `json:"chat_instance"`

	// (Optional) For widgets in inline messages, type of the chat
	// the message was sent to.
	ChatType string `json:"chat_type,omitempty"`

	// Type of the query.
	Type WidgetQueryType `json:"type"`

	// Data of the query, 1-64 bytes.
	Data string `json:"data"`

	// (Optional) Unique identifier of the shared widget that sent the query.
	WidgetID string `json:"widget_id,omitempty"`
}

// WidgetQueryType describes the type of a query from a widget.
type WidgetQueryType struct {
	// Type of the query: "data", "sign_data" or "sent_transaction".
	Type string `json:"type"`
}

// MessageSig satisfies Editable interface, so the message
// with the widget can be edited straight from its query.
func (q *WidgetQuery) MessageSig() (string, int64) {
	if q.InlineMessageID != "" || q.Chat == nil {
		return q.InlineMessageID, 0
	}
	return strconv.Itoa(q.MessageID), q.Chat.ID
}

// AnswerWidget sends a response for a given widget query. The data,
// up to 1024 bytes, is received only by the widget that sent the query
// and doesn't change the widget's own data.
func (b *Bot) AnswerWidget(query *WidgetQuery, data string) error {
	params := map[string]string{
		"widget_query_id": query.ID,
		"data":            data,
	}

	_, err := b.Raw("answerWidgetQuery", params)
	return err
}

// EditWidgetChatData replaces the message-specific data of a widget
// in an already sent message. An empty string removes the data.
// This function will panic upon nil Editable.
func (b *Bot) EditWidgetChatData(msg Editable, data string) error {
	msgID, chatID := msg.MessageSig()

	params := map[string]string{
		"chat_data": data,
	}

	if chatID == 0 { // if inline message
		params["inline_message_id"] = msgID
	} else {
		params["chat_id"] = strconv.FormatInt(chatID, 10)
		params["message_id"] = msgID
	}

	_, err := b.Raw("editMessageWidgetData", params)
	return err
}
