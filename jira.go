package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/wsxiaoys/terminal/color"
	//"github.com/kless/terminal"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type JiraResults struct {
	StartAt     int
	MaxResults  int
	Total       int
	Issues      []JiraIssue
}

type JiraIssue struct {
	Id     string
	Key    string
	Fields JiraIssueFields
}

type JiraIssueFields struct {
	Summary           string
	Description       *string
	Labels            *[]string
	ResolutionComment *string `json:"customfield_10016"`
	Status            JiraIssueStatus
	Comment           JiraCommentColl
	Attachment        []JiraAttachment
	Reporter          *JiraUser
	Assignee          *JiraUser
}

type JiraCommentColl struct {
	Comments []JiraComment
}

type JiraComment struct {
	Id      string
	Author  JiraUser
	Body    string
	Created string
}

type JiraAttachment struct {
	Id      string
	Content string
}

type JiraUser struct {
	Name        string
	DisplayName string
}

type JiraIssueStatus struct {
	Name string
}

type actionFn func(args []string)
type Action struct {
	ActionFunc actionFn
	Desc       string
}

var browseBaseUrl = "https://bits.bazaarvoice.com/jira/browse"
var baseUrl = "https://bits.bazaarvoice.com/jira/rest/api/2"
var assignedSearchQuery = "assignee = currentUser() ORDER BY status ASC, key DESC"
var watchedSearchQuery = "issue in watchedIssues() AND status != Closed ORDER BY status ASC, key DESC"

var actionMap = map[string]Action{
	"watched": Action{
		listWatched,
		"list watched tickets"},
	"comments": Action{
		showComments,
		"show comments for a ticket"},
	"add-comment": Action{
		addComment,
		"add a comment to a ticket"},
	"desc": Action{
		showDesc,
		"show the description of a ticket"},
	"assign": Action{
		reassignIssue,
		"reassign a ticket to another user or unassign if no user is specified"},
	"attachments": Action{
		downloadAttachments,
		"download a tickets attachments to the current directory"},
	"link": Action{
		ticketLink,
		"print a link to a ticket"},
	"open": Action{
		openTicket,
		"open a ticket in the browser"},
}

var useColor = false
var termWidth = 80

/* --- Actions --- */

func openTicket(args []string) {
	var err error
	if len(args) != 1 {
		fmt.Println("Need ticket number to open")
		return
	}
	url := browseBaseUrl + "/" + args[0]
	openBrowser(url)
	if err != nil {
		fmt.Println(err)
	}
	return
}

func ticketLink(args []string) {
	if len(args) != 1 {
		fmt.Println("Need a ticket number")
		return
	}
	fmt.Println(browseBaseUrl + "/" + args[0])
	return
}

func reassignIssue(args []string) {
	if len(args) < 1 {
		fmt.Println("Need a ticket number and ldap name")
		return
	}

	url := baseUrl + "/issue/" + args[0] + "/assignee"

	var err error
	var rawPayload []byte
	var assignee *string
	if len(args) == 2 {
		assignee = &args[1]
	} else {
		assignee = nil
	}

	payload := map[string]*string{
		"name": assignee,
	}
	if rawPayload, err = json.Marshal(payload); err != nil {
		fmt.Println(err)
		return
	}

	status, err := makeRequest("PUT", url, rawPayload)
	if err != nil {
		fmt.Println(err)
		return
	}
	if status != 204 {
		fmt.Println(status)
		return
	}

	if assignee != nil {
		fmt.Printf("Reassigned %s to %s\n", strings.ToUpper(args[0]), *assignee)
	} else {
		fmt.Printf("Unassigned %s\n", strings.ToUpper(args[0]))
	}
}

func listWatched(args []string) {
	url := makeSearchUrl(watchedSearchQuery)
	body, err := makeGetRequest(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	smartPrintln("@yWatched Issues@|")
	displayResults(body)
}

func showComments(args []string) {
	if len(args) != 1 {
		fmt.Println("Need a ticket number to display comments for")
		return
	}
	url := baseUrl + "/issue/" + args[0]
	body, err := makeGetRequest(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	displayComments(body)
}

func addComment(args []string) {
	if len(args) != 2 {
		fmt.Println("Need a ticket number and a comment message to add")
		return
	}
	url := baseUrl + "/issue/" + args[0] + "/comment"

	payload := map[string]string{
		"body": args[1],
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	status, err := makeRequest("POST", url, rawPayload)
	if err != nil {
		fmt.Println(err)
		return
	}
	if status != 201 {
		fmt.Println(status)
		return
	}

	fmt.Printf("Comment added to %s\n", strings.ToUpper(args[0]))
}

func showDesc(args []string) {
	if len(args) != 1 {
		fmt.Println("Need a ticket number to display description")
		return
	}
	url := baseUrl + "/issue/" + args[0]
	body, err := makeGetRequest(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	var issue JiraIssue
	if err = json.Unmarshal(body, &issue); err != nil {
		fmt.Println("Error unmarshaling json")
		fmt.Println(err)
		return
	}

	smartPrintf("@c%s @|- @y%s@| - @b%s@|\n", issue.Key, issue.Fields.Summary, issue.Fields.Status.Name)
	smartPrintf("@mReporter@|: %s @r<%s>@|\n", issue.Fields.Reporter.DisplayName, issue.Fields.Reporter.Name)
	if issue.Fields.Assignee != nil {
		smartPrintf("@gAssignee@|: %s @r<%s>@|\n", issue.Fields.Assignee.DisplayName, issue.Fields.Assignee.Name)
	} else {
		smartPrintln("@gAssignee@|: @rUnassigned@|")
	}

	if issue.Fields.Labels != nil {
		smartPrintf("@cLabels@|: %s\n", strings.Join(*issue.Fields.Labels, ", "))
	}

	if issue.Fields.Description == nil {
		fmt.Println("No description")
		return
	} else {
		smartPrintln("\n@yDescription@|")
		fmt.Println(*issue.Fields.Description)
	}

	if issue.Fields.ResolutionComment != nil {
		smartPrintln("\n@yResolution@|")
		fmt.Println(*issue.Fields.ResolutionComment)
	}

	if len(issue.Fields.Attachment) != 0 {
		smartPrintln("\n@yAttachments@|")
		for _, attachment := range issue.Fields.Attachment {
			url = attachment.Content
			parts := strings.Split(url, "/")
			baseName := parts[len(parts)-1]
			fmt.Println(baseName)
		}
	}
}

func downloadAttachments(args []string) {
	if len(args) != 1 {
		fmt.Println("Need a ticket number to fetch attachments")
		return
	}
	url := baseUrl + "/issue/" + args[0]
	body, err := makeGetRequest(url)
	if err != nil {
		fmt.Println(err)
		return
	}

	var issue JiraIssue
	err = json.Unmarshal(body, &issue)
	if err != nil {
		fmt.Println(err)
	}

	if len(issue.Fields.Attachment) == 0 {
		fmt.Println("No attachments")
		return
	}

	smartPrintln("@yDownloading attachments...@|")
	for _, attachment := range issue.Fields.Attachment {
		url = attachment.Content
		parts := strings.Split(url, "/")
		baseName := parts[len(parts)-1]

		fmt.Println(baseName)
		body, err := makeGetRequest(url)
		if err != nil {
			fmt.Println(err)
			return
		}
		file, err := os.Create(baseName)
		if err != nil {
			fmt.Println(err)
			return
		}
		defer file.Close()
		file.Write(body)
	}
}

func listAssigned() {
	searchUrl := makeSearchUrl(assignedSearchQuery)
	body, err := makeGetRequest(searchUrl)
	if err != nil {
		fmt.Println(err)
		return
	}

	smartPrintln("@yAssigned Issues@|")
	displayResults(body)
}

/* --- Utility Functions --- */

// uses color if availible
func smartPrintf(format string, a ...interface{}) (cnt int, err error) {
	pattern := regexp.MustCompile(`@.`)
	if useColor {
		cnt, err = color.Printf(format, a...)
	} else {
		rawFormat := pattern.ReplaceAll([]byte(format), []byte(""))
		cnt, err = fmt.Printf(string(rawFormat), a...)
	}
	return
}

// uses color if availible
func smartPrintln(str string) (cnt int, err error) {
	pattern := regexp.MustCompile(`@.`)
	if useColor {
		cnt, err = color.Println(str)
	} else {
		rawStr := pattern.ReplaceAll([]byte(str), []byte(""))
		cnt, err = fmt.Println(string(rawStr))
	}
	return
}

func openBrowser(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		// I don't care about windows
		err = fmt.Errorf("unsupported platform")
	}
	return
}

func makeGetRequest(url string) (respBody []byte, err error) {
	username, passwd, err := loadCredentials()
	if err != nil {
		return
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}

	encoded := encodeAuth(username, passwd)
	req.Header.Add("Authorization", "Basic "+encoded)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println(url)
		fmt.Println(resp.Status)
		return
	}

	respBody, err = ioutil.ReadAll(resp.Body)
	return
}

func makeRequest(method string, url string, payload []byte) (status int, err error) {
	username, passwd, err := loadCredentials()
	if err != nil {
		return
	}

	payloadReader := bytes.NewReader(payload)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payloadReader)
	if err != nil {
		return
	}

	encoded := encodeAuth(username, passwd)
	req.Header.Add("Authorization", "Basic "+encoded)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	status = resp.StatusCode
	return
}

func displayResults(body []byte) {
	var results JiraResults
	if err := json.Unmarshal(body, &results); err != nil {
		fmt.Println(err)
		return
	}

	if results.Issues == nil {
		fmt.Println("No results")
		return
	}

	for _, issue := range results.Issues {
		smartPrintf("@c%10s\t@b%12s\t@|%s\n", issue.Key,
			issue.Fields.Status.Name, issue.Fields.Summary)
	}
	return
}

func displayComments(body []byte) {
	var issue JiraIssue
	if err := json.Unmarshal(body, &issue); err != nil {
		fmt.Println(err)
		return
	}

	smartPrintf("@c%s @|- @y%s@| - @b%s@|\n", issue.Key, issue.Fields.Summary,
		issue.Fields.Status.Name)

	if len(issue.Fields.Comment.Comments) == 0 {
		fmt.Println("No comments")
		return
	}

	for _, comment := range issue.Fields.Comment.Comments {
		parts := strings.Split(comment.Created, ".")
		timestamp, err := time.Parse("2006-01-02T15:04:05", parts[0])
		if err != nil {
			fmt.Println(err)
			return
		}

		smartPrintf("@b%v %v %v %02v:%02v@|\n", timestamp.Day(),
			timestamp.Month(), timestamp.Year(), timestamp.Hour(),
			timestamp.Minute())
		smartPrintf("@gAuthor: @|%s @r<%s>@|\n", comment.Author.DisplayName,
			comment.Author.Name)
		smartPrintf("%s\n\n", comment.Body)
	}
}

func makeSearchUrl(query string) (searchUrl string) {
	encQuery := url.QueryEscape(query)
	searchUrl = baseUrl + "/search?jql=" + encQuery
	return
}

func encodeAuth(username, passwd string) (encoded string) {
	unencoded := username + ":" + passwd
	encoded = base64.StdEncoding.EncodeToString([]byte(unencoded))
	return
}

func loadCredentials() (username, passwd string, err error) {
	// not the most secure way to store the credentials, but it works
	homeDir := os.Getenv("HOME")
	fin, err := os.Open(homeDir + "/.jirarc")
	if err != nil {
		fmt.Println("Missing JIRA credentials in ~/.jirarc")
		os.Exit(1)
	}
	rawContent, err := ioutil.ReadAll(fin)
	content := strings.TrimSpace(string(rawContent))
	credentials := strings.Split(content, ":")
	username = credentials[0]
	passwd = credentials[1]
	return
}

func main() {
	//	if terminal.SupportANSI() {
	//		useColor = false
	//	}
	useColor = true

	if len(os.Args) >= 2 {
		// have to special case the "help" command to avoid init loop on actionMap
		if os.Args[1] == "help" {
			fmt.Println("The following commands are availible\n")
			for cmd, action := range actionMap {
				fmt.Printf("%-12s\t%s\n", cmd, action.Desc)
			}
			return
		}
		if action, ok := actionMap[os.Args[1]]; ok {
			action.ActionFunc(os.Args[2:])
		} else {
			showDesc(os.Args[1:])
		}
	} else {
		listAssigned()
	}
}
