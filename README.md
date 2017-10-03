[![CircleCI](https://circleci.com/gh/k-saka/slack-haven.svg?style=svg)](https://circleci.com/gh/k-saka/slack-haven)

# slack-haven
`slack-haven` relays message which posted on slack channel to other slack channel. 
This feature makes group DMs contained unlimited people(Usually slack group DMs contains up to 9 people).
It's useful for restricted creating private channel environment.
 

## Installation
`go get github.com/k-saka/slack-haven`

## Usage
1. Create group DMs which you want to relay on slack web

2. Check slack channel ID  
It's contained by slack web url.
If `https://slack.com/messages/XYXYXY/` is channel url, `XYXYXY` is channel ID.

3. Start bot  
`slack-haven -channel CHANNEL_X,CHANNEL_Y -token SLACK_TOKEN -log info`
  - `channel` [requirement]  
    comma separated slack group DM ID text
  - `token` [requirement]  
    slack api token
  - `log`
    loglevel

## Configuration file
`slack-haven` supports reading configuration from file. It uses `.slack-haven`file  located home directory. Configuration file format is JSON. 
Enable key
- `token`
  slack api token text
- `relay-rooms`
  group DM ID array

Example of `.slack-haven`  
`{"token": "SLACK_TOKEN", "relay-rooms": ["CHANNEL_X", "CHANNEL_Y"]}`
