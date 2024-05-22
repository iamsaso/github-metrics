package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

var rateLimitReset int64
var mu sync.Mutex

type UserMetrics struct {
	Commits int
	HoC     int
	Issues  int
	LcP     float64
	Msgs    int
	Pulls   int
	Reviews int
	Score   float64
}

type UserMetricsView struct {
	User    string
	Metrics UserMetrics
	CreatedSince string
	Organization string
}

func main() {
	var token string
	var days int
	var coders coderList
	var repos repoList
	var verbose bool
	var metric string
	var delay int
	var organization string

	// Define flags
	flag.StringVar(&token, "token", "", "GitHub token")
	flag.IntVar(&days, "days", 30, "Number of days to measure")
	flag.Var(&coders, "coder", "GitHub usernames to measure (can be specified multiple times)")
	flag.Var(&repos, "repo", "GitHub repositories to measure (can be specified multiple times)")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&metric, "metric", "all", "Specific metric to calculate (commits, hoc, issues, lcp, msgs, pulls, reviews, score)")
	flag.IntVar(&delay, "delay", 30, "Delay between API calls in seconds")
	flag.StringVar(&organization, "organization", "", "GitHub organization to filter repositories")

	// Check for .githubmetrics file
	if _, err := os.Stat(".githubmetrics"); err == nil {
		file, err := os.Open(".githubmetrics")
		if err != nil {
			log.Fatalf("Error opening .githubmetrics file: %v", err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				// Split the line into key and value
				keyValue := strings.SplitN(line, "=", 2)
				if len(keyValue) != 2 {
					continue
				}
				key, value := keyValue[0], keyValue[1]

				// Manually set the flags using flag.CommandLine.Set
				switch key {
				case "--token":
					flag.CommandLine.Set("token", value)
				case "--days":
					flag.CommandLine.Set("days", value)
				case "--coder":
					coders.Set(value)
				case "--repo":
					repos.Set(value)
				case "--verbose":
					flag.CommandLine.Set("verbose", value)
				case "--metric":
					flag.CommandLine.Set("metric", value)
				case "--delay":
					flag.CommandLine.Set("delay", value)
				case "--organization":
					flag.CommandLine.Set("organization", value)
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading .githubmetrics file: %v", err)
		}
	}

	// Parse command-line flags
	flag.Parse()

	if len(repos) == 0 && organization == "" {
		log.Fatal("No repositories or organization specified. Use --repo to add repositories or --organization to filter by organization.")
	}

	client := createGitHubClient(token)
	metrics := calculateMetrics(client, coders, days, metric, delay, organization, verbose)

	err := renderTemplate(metrics, days, organization)
	if err != nil {
		log.Fatalf("Error rendering template: %v", err)
	}
}

// coderList is a custom flag.Value implementation to handle multiple coders
type coderList []string

func (c *coderList) String() string {
	return fmt.Sprint(*c)
}

func (c *coderList) Set(value string) error {
	*c = append(*c, value)
	return nil
}

// repoList is a custom flag.Value implementation to handle multiple repositories
type repoList []string

func (r *repoList) String() string {
	return fmt.Sprint(*r)
}

func (r *repoList) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func createGitHubClient(token string) *github.Client {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func calculateMetrics(client *github.Client, users []string, days int, metric string, delay int, organization string, verbose bool) map[string]UserMetrics {
	if verbose {
		log.Printf("Calculating %s metric for %d users for %d days\n", metric, len(users), days)
	}
	metrics := make(map[string]UserMetrics)
	for _, user := range users {
		repos := getUserRepositories(client, user, days, delay, organization, verbose)
		fmt.Printf("User %s has %d repositories\n", user, len(repos))
		for _, repoFullName := range repos {
			owner, repoName := parseRepo(repoFullName)
			if owner == "" || repoName == "" {
				log.Printf("Skipping invalid repo string: %s", repoFullName)
				continue
			}

			switch metric {
			case "commits":
				commits := getCommits(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Commits: commits})
			case "hoc":
				hoc := getHoC(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{HoC: hoc})
			case "issues":
				issues := getIssues(client, owner, repoName, user, days, delay)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Issues: issues})
			case "lcp":
				lcp := getLcP(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{LcP: lcp})
			case "msgs":
				msgs := getMsgs(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Msgs: msgs})
			case "pulls":
				pulls := getPulls(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Pulls: pulls})
			case "reviews":
				reviews := getReviews(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Reviews: reviews})
			case "all":
				commits := getCommits(client, owner, repoName, user, days, delay, verbose)
				hoc := getHoC(client, owner, repoName, user, days, delay, verbose)
				issues := getIssues(client, owner, repoName, user, days, delay)
				lcp := getLcP(client, owner, repoName, user, days, delay, verbose)
				msgs := getMsgs(client, owner, repoName, user, days, delay, verbose)
				pulls := getPulls(client, owner, repoName, user, days, delay, verbose)
				reviews := getReviews(client, owner, repoName, user, days, delay, verbose)
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{
					Commits: commits, HoC: hoc, Issues: issues, LcP: lcp, Msgs: msgs, Pulls: pulls, Reviews: reviews,
				})
				metrics[user] = updateUserMetrics(metrics[user], UserMetrics{Score: calculateScore(metrics[user])})
			default:
				log.Fatalf("Unknown metric: %s", metric)
			}
		}
	}

	return metrics
}

func retryWithBackoff(ctx context.Context, attempts int, delay time.Duration, fn func() (interface{}, *github.Response, error)) (interface{}, *github.Response, error) {
	var err error

	for i := 0; i < attempts; i++ {
		var result interface{}
		var resp *github.Response

		result, resp, err = fn()

		if err == nil {
			return result, resp, nil
		}

		log.Printf("Attempt %d failed with error: %v", i+1, err)

		if resp != nil {
			if resp.StatusCode == 403 {
				sleepDuration := time.Until(time.Unix(resp.Rate.Reset.Unix(), 0))
				log.Printf("Rate limit exceeded. Sleeping until rate limit reset at %v", time.Unix(resp.Rate.Reset.Unix(), 0))
				time.Sleep(sleepDuration + delay) // Adding extra buffer time
			}
		}
	}

	return nil, nil, err
}

func updateUserMetrics(metrics, update UserMetrics) UserMetrics {
	metrics.Commits += update.Commits
	metrics.HoC += update.HoC
	metrics.Issues += update.Issues
	metrics.LcP += update.LcP
	metrics.Msgs += update.Msgs
	metrics.Pulls += update.Pulls
	metrics.Reviews += update.Reviews
	metrics.Score += update.Score
	return metrics
}

func calculateScore(metrics UserMetrics) float64 {
	return float64(metrics.HoC) + float64(metrics.Pulls)*250 + float64(metrics.Issues)*50 + float64(metrics.Commits)*5 + float64(metrics.Reviews)*150 + float64(metrics.Msgs)*5
}

func renderTemplate(metrics map[string]UserMetrics, days int, organization string) error {
	var sortedMetrics []UserMetricsView
	for user, metric := range metrics {
		sortedMetrics = append(sortedMetrics, UserMetricsView{
			User: user, 
			Metrics: metric, 
			CreatedSince: time.Now().AddDate(0, 0, -days).Format("2006-01-02"),
			Organization: organization,
		})
	}

	sort.Slice(sortedMetrics, func(i, j int) bool {
		return sortedMetrics[i].Metrics.Score > sortedMetrics[j].Metrics.Score
	})

	tmpl, err := template.ParseFiles("template.html")
	if err != nil {
		return err
	}

	file, err := os.Create("metrics.html")
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, sortedMetrics)
}

func parseRepo(repo string) (string, string) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func getCommits(client *github.Client, owner, repo, user string, days, delay int, verbose bool) int {
	ctx := context.Background()
	commits := 0
	opts := &github.CommitsListOptions{
		Author: user,
		Since:  time.Now().AddDate(0, 0, -days),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Repositories.ListCommits(ctx, owner, repo, opts)
		})
		if err != nil {
			log.Printf("Error fetching commits for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return commits
		}
		commitList := result.([]*github.RepositoryCommit)
		for _, commit := range commitList {
			if commit.Author != nil && commit.Author.GetLogin() == user && !isMergeCommit(commit) {
				commits++
				if verbose {
					log.Printf("Found commit %s by %s in repo %s/%s\n", commit.GetSHA(), user, owner, repo)
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return commits
}

func getHoC(client *github.Client, owner, repo, user string, days, delay int, verbose bool) int {
	ctx := context.Background()
	hoc := 0
	opts := &github.CommitsListOptions{
		Author: user,
		Since:  time.Now().AddDate(0, 0, -days),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Repositories.ListCommits(ctx, owner, repo, opts)
		})
		if err != nil {
			log.Printf("Error fetching commits for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return hoc
		}
		commitList := result.([]*github.RepositoryCommit)
		for _, commit := range commitList {
			if commit.Author != nil && commit.Author.GetLogin() == user && !isMergeCommit(commit) {
				details, _, err := client.Repositories.GetCommit(ctx, owner, repo, commit.GetSHA(), nil)
				if err != nil {
					log.Printf("Error fetching commit details for commit %s: %v\n", commit.GetSHA(), err)
					continue
				}
				for _, file := range details.Files {
					hoc += file.GetAdditions() + file.GetChanges()
					if verbose {
						log.Printf("Commit %s: file %s - additions: %d, changes: %d\n", commit.GetSHA(), file.GetFilename(), file.GetAdditions(), file.GetChanges())
					}
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return hoc
}

func getIssues(client *github.Client, owner, repo, user string, days, delay int) int {
	ctx := context.Background()
	issues := 0
	opts := &github.IssueListByRepoOptions{
		Creator: user,
		Since:   time.Now().AddDate(0, 0, -days),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Issues.ListByRepo(ctx, owner, repo, opts)
		})
		if err != nil {
			log.Printf("Error fetching issues for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return issues
		}
		issueList := result.([]*github.Issue)
		for _, issue := range issueList {
			if !issue.IsPullRequest() {
				issues++
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return issues
}

func getLcP(client *github.Client, owner, repo, user string, days, delay int, verbose bool) float64 {
	ctx := context.Background()
	totalTime := 0.0
	count := 0
	opts := &github.IssueListByRepoOptions{
		Creator: user,
		State:   "closed",
		Since:   time.Now().AddDate(0, 0, -days),
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Issues.ListByRepo(ctx, owner, repo, opts)
		})
		if err != nil {
			log.Printf("Error fetching issues for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return 0.0
		}
		issues := result.([]*github.Issue)
		for _, issue := range issues {
			if issue.IsPullRequest() && issue.CreatedAt != nil && issue.ClosedAt != nil {
				duration := issue.ClosedAt.Sub(*&issue.CreatedAt.Time).Hours()
				totalTime += duration
				count++
				if verbose {
					log.Printf("Pull request #%d by %s: created at %s, closed at %s, duration: %.2f hours\n", issue.GetNumber(), user, issue.CreatedAt.String(), issue.ClosedAt.String(), duration)
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if count == 0 {
		return 0.0
	}

	averageLifecycle := totalTime / float64(count)
	if verbose {
		log.Printf("Average lifecycle of pull requests for user %s in repo %s/%s over the last %d days: %.2f hours\n", user, owner, repo, days, averageLifecycle)
	}
	return averageLifecycle
}

func getMsgs(client *github.Client, owner, repo, user string, days, delay int, verbose bool) int {
	ctx := context.Background()
	msgs := 0
	query := fmt.Sprintf("repo:%s/%s is:pr commenter:%s created:>%s", owner, repo, user, time.Now().AddDate(0, 0, -days).Format("2006-01-02"))
	opts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, opts)
		})
		if err != nil {
			log.Printf("Error fetching pull request comments for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return msgs
		}
		issues := result.(*github.IssuesSearchResult)
		for _, pr := range issues.Issues {
			msgs += pr.GetComments()
			if verbose {
				log.Printf("Pull request #%d by %s in repo %s/%s has %d comments\n", pr.GetNumber(), user, owner, repo, pr.GetComments())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return msgs
}

func getPulls(client *github.Client, owner, repo, user string, days, delay int, verbose bool) int {
	ctx := context.Background()
	pulls := 0
	query := fmt.Sprintf("repo:%s/%s is:pr author:%s merged:>%s", owner, repo, user, time.Now().AddDate(0, 0, -days).Format("2006-01-02"))
	opts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, opts)
		})
		if err != nil {
			log.Printf("Error fetching pull requests for user %s in repo %s/%s: %v\n", user, owner, repo, err)
			return pulls
		}
		issues := result.(*github.IssuesSearchResult)
		for _, issue := range issues.Issues {
			if issue.IsPullRequest() && issue.ClosedAt != nil {
				pulls++
				if verbose {
					log.Printf("Pull request #%d by %s in repo %s/%s was merged at %s\n", issue.GetNumber(), user, owner, repo, issue.ClosedAt.String())
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return pulls
}

func getReviews(client *github.Client, owner, repo, user string, days, delay int, verbose bool) int {
	ctx := context.Background()
	reviewsCount := 0
	query := fmt.Sprintf("repo:%s/%s reviewed-by:%s is:pr merged:>%s", owner, repo, user, time.Now().AddDate(0, 0, -days).Format("2006-01-02"))
	opts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, opts)
		})
		issues := result.(*github.IssuesSearchResult)
		if err != nil {
			log.Printf("Error fetching reviewed pull requests for user %s in repo %s/%s: %v\n", user, err)
			return reviewsCount
		}
		for _, issue := range issues.Issues {
			reviewsCount++
			if verbose {
				log.Printf("Pull request #%d reviewed by %s in repo %s/%s was merged at %s\n", issue.GetNumber(), user, owner, repo, issue.ClosedAt.String())
			}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return reviewsCount
}
func isMergeCommit(commit *github.RepositoryCommit) bool {
	return commit.Parents != nil && len(commit.Parents) > 1
}

func getUserRepositories(client *github.Client, user string, days, delay int, organization string, verbose bool) []string {
	ctx := context.Background()
	reposMap := make(map[string]bool)
	since := time.Now().AddDate(0, 0, -days)

	// Get repositories where the user created pull requests
	query := fmt.Sprintf("author:%s created:>%s", user, since)
	searchOpts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, searchOpts)
		})
		if err != nil {
			log.Printf("Error fetching pull requests commented by user %s: %v\n", user, err)
			break
		}
		issues := result.(*github.IssuesSearchResult)
		for _, issue := range issues.Issues {
			if issue.IsPullRequest() {
				repoFullName := parseRepoURL(issue.GetRepositoryURL())
				if repoFullName != "" && (organization == "" || strings.HasPrefix(repoFullName, organization+"/")) {
					reposMap[repoFullName] = true
					if verbose {
						log.Printf("User %s created pull request in repository %s\n", user, repoFullName)
					}
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		searchOpts.Page = resp.NextPage
	}

	// Get repositories where the user commented on pull requests
	query = fmt.Sprintf("commenter:%s created:>%s", user, since.Format("2006-01-02"))
	searchOpts = &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}
	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, searchOpts)
		})
		if err != nil {
			log.Printf("Error fetching pull requests commented by user %s: %v\n", user, err)
			break
		}
		issues := result.(*github.IssuesSearchResult)
		for _, issue := range issues.Issues {
			if issue.IsPullRequest() {
				repoFullName := parseRepoURL(issue.GetRepositoryURL())
				if repoFullName != "" && (organization == "" || strings.HasPrefix(repoFullName, organization+"/")) {
					reposMap[repoFullName] = true
					if verbose {
						log.Printf("User %s commented on pull request in repository %s\n", user, repoFullName)
					}
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		searchOpts.Page = resp.NextPage
	}

	// Get repositories where the user reviewed pull requests
	query = fmt.Sprintf("reviewed-by:%s created:>%s", user, since.Format("2006-01-02"))
	for {
		result, resp, err := retryWithBackoff(ctx, 5, time.Second, func() (interface{}, *github.Response, error) {
			return client.Search.Issues(ctx, query, searchOpts)
		})
		if err != nil {
			log.Printf("Error fetching pull requests reviewed by user %s: %v\n", user, err)
			break
		}
		issues := result.(*github.IssuesSearchResult)
		for _, issue := range issues.Issues {
			if issue.IsPullRequest() {
				repoFullName := parseRepoURL(issue.GetRepositoryURL())
				if repoFullName != "" && (organization == "" || strings.HasPrefix(repoFullName, organization+"/")) {
					reposMap[repoFullName] = true
					if verbose {
						log.Printf("User %s reviewed pull request in repository %s\n", user, repoFullName)
					}
				}
			}
		}
		if resp.NextPage == 0 {
			break
		}
		searchOpts.Page = resp.NextPage
	}

	// Convert map keys to slice
	var reposList []string
	for repo := range reposMap {
		reposList = append(reposList, repo)
	}

	return reposList
}

func parseRepoURL(repoURL string) string {
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return ""
	}
	return fmt.Sprintf("%s/%s", parts[len(parts)-2], parts[len(parts)-1])
}
