Jira Command Line Tool
====

I created this tool to manage JIRA tickets.   The tool expects a .jirarc file in your home directory that contains
your jira login and your jira password separated by a colon.

## Installation

Make sure you GOPATH is set

   go get "github.com/Unknwon/goconfig"
   go get "github.com/wsxiaoys/terminal/color"
   go build jira.go

## Setup

 Create config file

*Example .jirarc file*
~~~~
[default]
url=https://mine.atlassian.net
username=jdyer
password=foo
~~~~


Author:
* Jeremy Shoemaker
Contributers:
* John Dyer ( johntdyer(at)gmail.com )
