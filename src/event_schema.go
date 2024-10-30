package main

type Repository struct {
	FullName    string `json:"full_name"`
	ID          int    `json:"id"`
	NodeID      string `json:"node_id"`
	Owner       User   `json:"owner"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Fork        bool   `json:"fork"`
	URL         string `json:"url"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Committer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Commit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp string    `json:"timestamp"`
	Author    Author    `json:"author"`
	Committer Committer `json:"committer"`
	URL       string    `json:"url"`
}

type Pusher struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type CommitPushedEvent struct {
	Ref        string     `json:"ref"`
	Repository Repository `json:"repository"`
	Commits    []Commit   `json:"commits"`
	Pusher     Pusher     `json:"pusher"`
}

type User struct {
	Login string `json:"login"`
	Email string `json:"email"`
}

type Branch struct {
	Ref string `json:"ref"`
}

type PullRequest struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	State string `json:"state"`
	User  User   `json:"user"`
	Head  Branch `json:"head"`
	Base  Branch `json:"base"`
}

type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type WorkflowRun struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	HeadBranch string `json:"head_branch"`
	RunNumber  int    `json:"run_number"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	RunAttempt int    `json:"run_attempt"`
	HTMLURL    string `json:"html_url"`
}

type WorkflowRunEvent struct {
	Action      string      `json:"action"`
	WorkflowRun WorkflowRun `json:"workflow_run"`
	Repository  Repository  `json:"repository"`
}

type WorkflowJob struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	RunID       int      `json:"run_id"`
	RunAttempt  int      `json:"run_attempt"`
	Status      string   `json:"status"`
	Conclusion  string   `json:"conclusion"`
	StartedAt   string   `json:"started_at"`
	CompletedAt string   `json:"completed_at"`
	Labels      []string `json:"labels"`
	RunnerID    string   `json:"runner_id"`
}

type WorkflowJobEvent struct {
	Action      string      `json:"action"`
	WorkflowJob WorkflowJob `json:"workflow_job"`
	Repository  Repository  `json:"repository"`
}
