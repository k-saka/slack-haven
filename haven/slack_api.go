package haven

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
)

const (
	rtmStartURL    = "https://slack.com/api/rtm.start"
	postMessageURL = "https://slack.com/api/chat.postMessage"
	uploadFileURL  = "https://slack.com/api/files.upload"
	fileInfoURL    = "https://slack.com/api/files.info"
	reactionAddURL = "https://slack.com/api/reactions.add"
)

func callSlackJSONAPI(url string, token string, payload interface{}) ([]byte, error) {
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	jsonReader := bytes.NewReader(jsonBytes)
	req, err := http.NewRequest("POST", url, jsonReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// StartAPI call slack rtm.start api
func StartAPI(token string) (resp *RTMStartResponse, err error) {
	payload := RTMStartRequest{SimpleLatest: true, NoUnreads: true}
	responseBytes, err := callSlackJSONAPI(rtmStartURL, token, payload)
	slackResponse := RTMStartResponse{}
	if err = json.Unmarshal(responseBytes, &slackResponse); err != nil {
		return nil, err
	}
	if !slackResponse.Ok {
		return nil, errors.New(slackResponse.Error)
	}
	// sort members
	for _, c := range slackResponse.Channels {
		sort.Strings(c.Members)
	}
	for _, c := range slackResponse.Groups {
		sort.Strings(c.Members)
	}

	return &slackResponse, nil
}

// postMessage send a message to slack throw chat.postMessage API
func postMessage(token string, pm PostMessage) (*PostMessageResponse, error) {
	responseBytes, err := callSlackJSONAPI(postMessageURL, token, pm)
	slackResponse := PostMessageResponse{}
	if err = json.Unmarshal(responseBytes, &slackResponse); err != nil {
		return nil, err
	}
	if !slackResponse.Ok {
		return nil, errors.New(slackResponse.Error)
	}
	return &slackResponse, nil
}

// add reaction
func addReaction(token string, ra ReactionAddRequest) (*SlackOk, error) {
	responseBytes, err := callSlackJSONAPI(reactionAddURL, token, ra)
	slackResponse := SlackOk{}
	if err = json.Unmarshal(responseBytes, &slackResponse); err != nil {
		return nil, err
	}
	if !slackResponse.Ok {
		return nil, errors.New(slackResponse.Error)
	}
	return &slackResponse, nil
}

func downloadFile(token, url string) (rc []byte, err error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return content, nil
}

func fetchFileInfo(token, id string) (f *File, err error) {
	req, err := http.NewRequest("GET", fileInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("file", id)
	req.URL.RawQuery = q.Encode()
	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	responseBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	slackResponse := FileInfo{}
	if err = json.Unmarshal(responseBytes, &slackResponse); err != nil {
		return nil, err
	}
	if !slackResponse.Ok {
		return nil, errors.New(slackResponse.Error)
	}
	return &slackResponse.File, nil
}

// uploadFile send file to slack
func uploadFile(token string, channels []string, content []byte, file *File) error {
	body := bytes.Buffer{}
	writer := multipart.NewWriter(&body)
	defer writer.Close()

	part, err := writer.CreateFormFile("file", file.Title)
	if err != nil {
		return err
	}

	part.Write(content)
	_ = writer.WriteField("token", token)
	_ = writer.WriteField("filetype", file.FileType)
	_ = writer.WriteField("filename", "botupload-"+file.Name)
	_ = writer.WriteField("channels", strings.Join(channels, ","))

	req, err := http.NewRequest("POST", uploadFileURL, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// json check
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	ok := &SlackOk{}
	if err := json.Unmarshal(respBody, ok); err != nil {
		return err
	}

	if !ok.Ok {
		return ok.NewError()
	}

	return nil
}
