## Pre-requisites
- Install latest go lang version and sqlite on local. If using mac, use brew commands.

## Starting the server
- Go to folder where repo is checked out
- Execute `go mod tidy` to download missing imports based on package graph
- Execute `go run cmd/server/main.go` to start the server. By default port is 8080.

## How to use application
- Open `http://localhost:8080/` to access the mock login page. Use `admin/admin` as user/password
- This will login as a client.
- Use `Campaigns` tab to create campaigns. There are 3 ads which can be selected for every campaign. DMA can be `*` or any other valid number (PS: DMA is not validated for its accuracy)
- Once Campaign is created, it can be edited as well to reuse.
- Use `Client Demo` to simulate rendering ads using VAST. Client-Id is hardcoded for testing purposes. Change the DMA to render relevant ads. Using `*` as DMA will render all campaigns as long as campaign dates fall within the window.
- Once Ads are requested multiple times, and when threshold of 300s is reached, no more Ads will be served.

## Backend Details
- Every request and response data are persisted in RequestLog model
- Similarly impressions table has all details are ads served for every client
- VAST 3.0 structure has been utilized to create dynamic response data when rendering ads.

## DB Access
- Execute `sqlite3 adserver.db` to start a terminal to access DB
- Query request log  `select id, method, path, request_body, response_status from request_logs;`
- Query impressions `select * from impressions;`
- Query campaigns `select * from campaigns;`

