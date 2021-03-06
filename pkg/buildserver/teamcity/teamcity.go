package teamcity

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// TCClient is client object to talk to teamcity
type TCClient struct {
	client    *http.Client
	token     string
	serverURL string
}

// NewTeamcityClient ...
func NewTeamcityClient(
	requestTimeout, dialTimeout, tlsHandshakeTimeout time.Duration,
	serverURL, token string,
	insecure bool,
) *TCClient {
	tr := &http.Transport{
		Dial: (&net.Dialer{
			Timeout: dialTimeout,
		}).Dial,
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: tlsHandshakeTimeout,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: insecure},
	}

	client := &http.Client{
		Timeout:   requestTimeout,
		Transport: tr,
	}

	return &TCClient{
		client:    client,
		serverURL: serverURL,
		// Trim the bearer from the token, to keep the API backward compatible
		// with previous versions were the client had to add the Bearer to the
		// token beforehand.
		token: strings.TrimPrefix(token, "Bearer "),
	}
}

// GetBuild returns build details
// for the provided id
func (t *TCClient) GetBuild(id int, buildDetails interface{}) (err error) {

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/app/rest/builds/id:%d", t.serverURL, id), nil)
	if err != nil {
		return err
	}
	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return
	}

	err = json.Unmarshal(body, &buildDetails)
	if err != nil {
		log.Println(err.Error())
		return
	}

	return
}

/*
StartBuild adds a build to the build queue

buildTypeID is the unique ID of a build pipeline

branch is the branch name on which build will be triggered

params is a map containing env variables and other overrides that
user wants to provide
*/
func (t *TCClient) StartBuild(
	buildTypeID, branch, comment string,
	params map[string]string,
	snapshotDependencies map[string]int,
	artifactDependencies map[string]int) (int, error) {
	var buildDetails TCBuildDetails

	payload := TCBuildPayload{
		BuildType: TCBuildType{
			ID: buildTypeID,
		},
		Comment: TCBuildComment{
			Text: comment,
		},
		Properties: TCBuildProperties{
			Property: []TCBuildProperty{},
		},
		Personal:   "False",
		BranchName: branch,
	}

	// Add params to properties
	for k, v := range params {
		payload.Properties.Property = append(payload.Properties.Property, TCBuildProperty{k, v})
	}

	snapDeps := TCBuildSnapshotDependencies{
		Builds: []TCBuildDetails{},
	}
	artfDeps := TCBuildSnapshotDependencies{
		Builds: []TCBuildDetails{},
	}

	// Add snapshot dependencies to request
	for k, v := range snapshotDependencies {
		snapDeps.Builds = append(snapDeps.Builds, TCBuildDetails{ID: v, BuildTypeID: k})
	}

	// Add artifact dependencies to request
	for k, v := range artifactDependencies {
		artfDeps.Builds = append(artfDeps.Builds, TCBuildDetails{ID: v, BuildTypeID: k})
	}

	if len(snapDeps.Builds) > 0 {
		payload.SnapshotDependencies = &snapDeps
	}

	if len(artfDeps.Builds) > 0 {
		payload.ArtifactDependencies = &artfDeps
	}

	requestPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println(err.Error())
		return -1, err
	}

	log.Println(string(requestPayload))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/app/rest/buildQueue", t.serverURL),
		bytes.NewBuffer(requestPayload))
	if err != nil {
		return -1, err
	}
	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return -1, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return -1, err
	}

	log.Printf(string(body))
	err = json.Unmarshal(body, &buildDetails)
	if err != nil {
		log.Println(err.Error())
		return -1, err
	}

	log.Println(buildDetails)
	return buildDetails.ID, nil
}

// CancelQueuedBuild cancels a build that is currently
// queued in the BuildQueue
// If the build has already started or finished,
// this call will fail
func (t *TCClient) CancelQueuedBuild(id int, comment string) error {
	// var buildDetails TCBuildDetails

	payload := TCBuildStopPayload{
		Comment:        comment,
		ReaddIntoQueue: "false",
	}

	requestPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	log.Println(string(requestPayload))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/app/rest/buildQueue/%d", t.serverURL, id),
		bytes.NewBuffer(requestPayload))
	if err != nil {
		return err
	}
	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	/* err = json.Unmarshal(body, &buildDetails)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	log.Println(buildDetails) */
	log.Println(string(body))
	return nil
}

// StopBuild stops a running build
func (t *TCClient) StopBuild(id int, comment string) error {
	// var buildDetails TCBuildDetails

	payload := TCBuildStopPayload{
		Comment:        comment,
		ReaddIntoQueue: "false",
	}

	requestPayload, err := json.Marshal(payload)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	log.Println(string(requestPayload))

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/app/rest/builds/%d", t.serverURL, id),
		bytes.NewBuffer(requestPayload))
	if err != nil {
		return err
	}
	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return err
	}

	log.Println(string(body))
	return nil
}

/*
GetArtifactTextFile fetches the content of an artifact file

path is the relative path of the file in teamcity artifacts

id is the build id from which the artifact will be fetched

It returns content of the file as array of bytes, content type of that file and error object if any
*/
func (t *TCClient) GetArtifactTextFile(path string, id int) ([]byte, string, error) {
	var fileContent []byte
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/app/rest/builds/id:%d/artifacts/content/%s", t.serverURL, id, path), nil)
	if err != nil {
		return nil, "", err
	}
	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		log.Println(err.Error())
		return fileContent, "", err
	}

	defer resp.Body.Close()
	fileContent, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err.Error())
		return fileContent, "", err
	}
	return fileContent, resp.Header.Get("Content-Type"), nil
}

func (t *TCClient) setAuthorizationHeader(headers http.Header) {
	headers.Add("Authorization", fmt.Sprintf("Bearer %s", t.token))
}

// GetAllBuilds returns the list of builds as per the query params
// provided by user
func (t *TCClient) GetAllBuilds(params TCQueryParams) (builds TCBuildSnapshotDependencies, err error) {
	requestURL := fmt.Sprintf("%s/app/rest/builds/?locator=", t.serverURL)

	if params.BuildTypeID != "" {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("buildType:(id:%s),", params.BuildTypeID))
	}

	if params.Branch != "" {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("branch:(name:%s),", params.Branch))
	}

	if params.User != "" {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("user:%s,", params.User))
	}

	if params.Count > 0 {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("count:%d,", params.Count))
	}

	if params.Start > 0 {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("start:%d,", params.Start))
	}

	if params.LookupLimit > 0 {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("lookupLimit:%d,", params.LookupLimit))
	}

	if params.Running {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("running:%t,", params.Running))
	}

	if params.Cancelled {
		requestURL = fmt.Sprintf("%s%s", requestURL, fmt.Sprintf("cancelled:%t,", params.Cancelled))
	}

	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return
	}

	t.setAuthorizationHeader(req.Header)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if err = json.Unmarshal(respBody, &builds); err != nil {
		return
	}

	return
}
