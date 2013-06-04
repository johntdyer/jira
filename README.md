Jira Command Line Tool
====

I created this tool to manage JIRA tickets assigned to me while
working at Bazaarvoice.  At the moment, the API endpoint is hard-coded
but it should be easy to change that to be configuration based.

The tool expects a .jirarc file in your home directory that contains
your jira login and your jira password separated by a colon.

*Example .jirarc file*
~~~~
username:password
~~~~
