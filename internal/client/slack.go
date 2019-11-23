package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sharovik/devbot/internal/dto"
	"github.com/sharovik/devbot/internal/log"
	"golang.org/x/net/websocket"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

//SlackClient client for slack api calls
type SlackClient struct {
	Client     *http.Client
	BaseURL    string
	OAuthToken string
}

//SlackClientInterface interface for slack client
type SlackClientInterface interface {
	//Http methods
	request(string, string, []byte) ([]byte, int, error)
	Post(string, []byte) ([]byte, int, error)
	Get(string) ([]byte, int, error)
	Put(string, []byte) ([]byte, int, error)

	//Methods for slackAPI endpoints
	GetConversationsList() (dto.SlackResponseConversationsList, int, error)
	GetUsersList() (dto.SlackResponseUsersList, int, error)
	SendMessageToWs(*websocket.Conn, dto.SlackRequestEventMessage) error

	//PM messages
	SendMessage(dto.SlackRequestChatPostMessage) (dto.SlackResponseChatPostMessage, int, error)
}

func (client SlackClient) request(method string, endpoint string, body []byte) ([]byte, int, error) {

	log.Logger().StartMessage("Slack request")

	var resp *http.Response

	log.Logger().Info().
		Str("base_url", client.BaseURL).
		Str("endpoint", endpoint).
		Str("method", method).
		Msg("Endpoint call")

	request, err := http.NewRequest(method, client.BaseURL+endpoint, bytes.NewReader(body))
	if err != nil {
		log.Logger().AddError(err).Msg("Error during the request generation")
		log.Logger().FinishMessage("Slack request")
		return nil, 0, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", client.OAuthToken))
	resp, errorResponse := client.Client.Do(request)

	if resp == nil {
		err = errors.New("Response cannot be null ")
		errMsg := err.Error()
		if errorResponse != nil {
			errMsg = errorResponse.Error()
		}
		log.Logger().AddError(errorResponse).
			Str("response_error", errMsg).
			Msg("Error during response body parse")

		log.Logger().FinishMessage("Slack request")
		return nil, 0, err
	}

	//1. Parse the response body
	defer resp.Body.Close()
	byteResp, errorConversion := ioutil.ReadAll(resp.Body)
	if errorConversion != nil {
		log.Logger().AddError(errorConversion).
			Err(errorConversion).
			Msg("Error during response body parse")
		log.Logger().FinishMessage("Slack request")
		return byteResp, 0, errorConversion
	}

	var response []byte
	if string(byteResp) == "" {
		response = []byte(`{}`)
	} else {
		response = byteResp
	}

	//5. For status codes, which are equal or more then 400, we should return an error. But we must mark it as a warning, because sometimes bad status code related to the validation
	if resp.StatusCode >= http.StatusBadRequest {
		err = fmt.Errorf("Bad status code received: %d ", resp.StatusCode)
		log.Logger().Warn().Int("status_code", resp.StatusCode).
			Err(err).
			Str("response", string(response)).
			Msg("Bad status code received")
		log.Logger().FinishMessage("Slack request")
		return byteResp, resp.StatusCode, err
	}

	log.Logger().FinishMessage("Slack request")
	return byteResp, resp.StatusCode, nil
}

//Post method for POST http requests
func (client SlackClient) Post(endpoint string, body []byte) ([]byte, int, error) {
	return client.request(http.MethodPost, endpoint, body)
}

//Put method for PUT http requests
func (client SlackClient) Put(endpoint string, body []byte) ([]byte, int, error) {
	return client.request(http.MethodPut, endpoint, body)
}

//Get method for GET http requests
func (client SlackClient) Get(endpoint string) ([]byte, int, error) {
	return client.request(http.MethodGet, endpoint, []byte(``))
}

//SendMessageToWs sends message to selected WebSocket EventsAPI
func (client SlackClient) SendMessageToWs(ws *websocket.Conn, m dto.SlackRequestEventMessage) error {
	log.Logger().Debug().Interface("message", m).Msg("Send message to EventsAPI")
	var counter uint64
	m.Id = atomic.AddUint64(&counter, 1)
	return websocket.JSON.Send(ws, m)
}

//SendMessage method for post message send through simple API request
func (client SlackClient) SendMessage(message dto.SlackRequestChatPostMessage) (dto.SlackResponseChatPostMessage, int, error) {
	log.Logger().Debug().Interface("message", message).Msg("Start chat.postMessage")
	byteStr, err := json.Marshal(message)
	if err != nil {
		return dto.SlackResponseChatPostMessage{}, 0, err
	}

	response, statusCode, err := client.Post("/chat.postMessage", byteStr)
	if err != nil {
		log.Logger().AddError(err).
			RawJSON("response", response).
			Int("status_code", statusCode).
			Msg("Failed send message")
		return dto.SlackResponseChatPostMessage{}, statusCode, err
	}

	var dtoResponse dto.SlackResponseChatPostMessage
	if err := json.Unmarshal(response, &dtoResponse); err != nil {
		return dto.SlackResponseChatPostMessage{}, statusCode, err
	}

	if !dtoResponse.Ok {
		return dtoResponse, statusCode, errors.New(dtoResponse.Error)
	}

	log.Logger().Debug().Interface("message", message).Msg("Finish chat.postMessage")
	return dtoResponse, statusCode, nil
}

//GetConversationsList method which returns the conversations list of current workspace
func (client SlackClient) GetConversationsList() (dto.SlackResponseConversationsList, int, error) {
	response, statusCode, err := client.Get("/conversations.list")
	if err != nil {
		return dto.SlackResponseConversationsList{}, statusCode, err
	}

	var dtoResponse dto.SlackResponseConversationsList
	if err := json.Unmarshal(response, &dtoResponse); err != nil {
		return dto.SlackResponseConversationsList{}, statusCode, err
	}

	if !dtoResponse.Ok {
		return dtoResponse, statusCode, errors.New(dtoResponse.Error)
	}

	return dtoResponse, statusCode, nil
}

//GetUsersList method which returns the users list of current workspace
func (client SlackClient) GetUsersList() (dto.SlackResponseUsersList, int, error) {
	response, statusCode, err := client.Get("/users.list")
	if err != nil {
		return dto.SlackResponseUsersList{}, statusCode, err
	}

	var dtoResponse dto.SlackResponseUsersList
	if err := json.Unmarshal(response, &dtoResponse); err != nil {
		return dto.SlackResponseUsersList{}, statusCode, err
	}

	if !dtoResponse.Ok {
		return dtoResponse, statusCode, errors.New(dtoResponse.Error)
	}

	return dtoResponse, statusCode, nil
}