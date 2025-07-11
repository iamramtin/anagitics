package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PullRequest struct {
	Title        string     `json:"title"`
	State        string     `json:"state"`
	MergedAt     *time.Time `json:"merged_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Number       int        `json:"number"`
	Additions    int        `json:"additions"`
	Deletions    int        `json:"deletions"`
	ChangedFiles int        `json:"changed_files"`
	User         struct {
		Login string `json:"login"`
	} `json:"user"`
}

type PRFile struct {
	Filename  string `json:"filename"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
}

type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Author struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
		Message string `json:"message"`
	} `json:"commit"`
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
	} `json:"stats"`
}

type Review struct {
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submitted_at"`
	User        struct {
		Login string `json:"login"`
	} `json:"user"`
}

type Comment struct {
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
}

type Issue struct {
	Title     string     `json:"title"`
	State     string     `json:"state"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Comments int `json:"comments"`
}

type Analytics struct {
	RepoName           string
	TotalPRs           int
	MergedPRs          int
	OpenPRs            int
	RecentPRs          int
	TotalCommits       int
	RecentCommits      int
	CodeReviews        int
	RecentReviews      int
	IssueComments      int
	RecentComments     int
	LinesAdded         int
	LinesRemoved       int
	NetLinesChanged    int
	FilesChanged       int
	AvgMergeTime       time.Duration
	IssuesCreated      int
	IssuesClosed       int
	CommitFrequency    float64
	PRSuccessRate      float64
	CodeChurnRate      float64
	CollaborationScore int
	TotalActivity      float64
	HasActivity        bool
	
	// New enhanced metrics
	MonthlyBreakdown   map[string]MonthlyData
	TechnologyBreakdown map[string]int
	CodeQualityMetrics CodeQuality
	WorkClassification WorkClassification
}

type MonthlyData struct {
	Month        string
	Commits      int
	PRs          int
	Reviews      int
	Comments     int
	LinesAdded   int
	LinesRemoved int
}

type CodeQuality struct {
	TestFiles       int
	TestLinesAdded  int
	DocFiles        int
	DocLinesAdded   int
	ConfigFiles     int
	CIFiles         int
}

type WorkClassification struct {
	BugFixes     int
	Features     int
	Refactoring  int
	Documentation int
	Infrastructure int
}

const baseUrl = "https://api.github.com"

func analyzeTechnology(filename string) string {
	// Extract file extension and classify technology
	if strings.Contains(filename, ".") {
		parts := strings.Split(filename, ".")
		ext := parts[len(parts)-1]
		
		switch strings.ToLower(ext) {
		case "go":
			return "Go"
		case "js", "jsx":
			return "JavaScript"
		case "ts", "tsx":
			return "TypeScript"
		case "py":
			return "Python"
		case "java":
			return "Java"
		case "rb":
			return "Ruby"
		case "php":
			return "PHP"
		case "c", "cpp", "cc", "cxx":
			return "C/C++"
		case "rs":
			return "Rust"
		case "kt":
			return "Kotlin"
		case "swift":
			return "Swift"
		case "html":
			return "HTML"
		case "css", "scss", "sass":
			return "CSS"
		case "sql":
			return "SQL"
		case "sh", "bash":
			return "Shell"
		case "yml", "yaml":
			return "YAML"
		case "json":
			return "JSON"
		case "xml":
			return "XML"
		case "md", "markdown":
			return "Markdown"
		case "tf":
			return "Terraform"
		case "dockerfile":
			return "Docker"
		}
	}
	
	// Special file patterns
	filename = strings.ToLower(filename)
	if strings.Contains(filename, "dockerfile") {
		return "Docker"
	}
	if strings.Contains(filename, "makefile") {
		return "Makefile"
	}
	if strings.Contains(filename, ".github") {
		return "GitHub Actions"
	}
	
	return "Other"
}

func classifyFile(filename string) (isTest bool, isDoc bool, isConfig bool, isCI bool) {
	filename = strings.ToLower(filename)
	
	// Test files
	isTest = strings.Contains(filename, "test") || 
		strings.Contains(filename, "_test.") || 
		strings.Contains(filename, ".spec.") ||
		strings.Contains(filename, ".test.") ||
		strings.Contains(filename, "__tests__") ||
		strings.HasSuffix(filename, "_test.go") ||
		strings.HasSuffix(filename, ".spec.js") ||
		strings.HasSuffix(filename, ".spec.ts")
	
	// Documentation files
	isDoc = strings.Contains(filename, "readme") ||
		strings.Contains(filename, "doc") ||
		strings.Contains(filename, "changelog") ||
		strings.HasSuffix(filename, ".md") ||
		strings.HasSuffix(filename, ".rst") ||
		strings.HasSuffix(filename, ".txt")
	
	// Configuration files
	isConfig = strings.Contains(filename, "config") ||
		strings.Contains(filename, ".env") ||
		strings.HasSuffix(filename, ".json") ||
		strings.HasSuffix(filename, ".yaml") ||
		strings.HasSuffix(filename, ".yml") ||
		strings.HasSuffix(filename, ".toml") ||
		strings.HasSuffix(filename, ".ini") ||
		strings.Contains(filename, "package.json") ||
		strings.Contains(filename, "go.mod")
	
	// CI/CD files
	isCI = strings.Contains(filename, ".github") ||
		strings.Contains(filename, ".gitlab-ci") ||
		strings.Contains(filename, "jenkins") ||
		strings.Contains(filename, "travis") ||
		strings.Contains(filename, "circle") ||
		strings.Contains(filename, "dockerfile") ||
		strings.Contains(filename, "docker-compose")
	
	return
}

func classifyCommitWork(message string) string {
	message = strings.ToLower(message)
	
	// Bug fixes
	if strings.Contains(message, "fix") || 
		strings.Contains(message, "bug") ||
		strings.Contains(message, "patch") ||
		strings.Contains(message, "hotfix") {
		return "BugFix"
	}
	
	// Features
	if strings.Contains(message, "feat") ||
		strings.Contains(message, "add") ||
		strings.Contains(message, "implement") ||
		strings.Contains(message, "new") {
		return "Feature"
	}
	
	// Refactoring
	if strings.Contains(message, "refactor") ||
		strings.Contains(message, "cleanup") ||
		strings.Contains(message, "optimize") ||
		strings.Contains(message, "improve") {
		return "Refactoring"
	}
	
	// Documentation
	if strings.Contains(message, "doc") ||
		strings.Contains(message, "readme") ||
		strings.Contains(message, "comment") {
		return "Documentation"
	}
	
	// Infrastructure
	if strings.Contains(message, "deploy") ||
		strings.Contains(message, "ci") ||
		strings.Contains(message, "build") ||
		strings.Contains(message, "docker") ||
		strings.Contains(message, "config") {
		return "Infrastructure"
	}
	
	return "Other"
}

func parseDuration(duration string) (time.Time, error) {
	re := regexp.MustCompile(`^(\d+)([dmy])$`)
	matches := re.FindStringSubmatch(duration)
	if len(matches) != 3 {
		return time.Time{}, fmt.Errorf("invalid duration format. Use format like 30d, 3m, 1y")
	}

	value, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, err
	}

	unit := matches[2]
	now := time.Now()

	switch unit {
	case "d":
		return now.AddDate(0, 0, -value), nil
	case "m":
		return now.AddDate(0, -value, 0), nil
	case "y":
		return now.AddDate(-value, 0, 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid unit. Use d, m, or y")
	}
}

func fetchOrgRepos(org string) []string {
	url := fmt.Sprintf("%s/orgs/%s/repos?type=all&per_page=100", baseUrl, org)
	repos := get[struct {
		FullName string `json:"full_name"`
	}](url)

	var repoNames []string
	for _, repo := range repos {
		repoNames = append(repoNames, repo.FullName)
	}
	return repoNames
}

func get[T any](url string) []T {
	client := &http.Client{}
	githubToken := os.Getenv("GH_TOKEN")
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("token %s", githubToken))
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to get %s: %v", url, err)
	}
	if resp.StatusCode >= 400 {
		log.Fatalf("github api error: %s", resp.Status)
	}

	defer resp.Body.Close()

	var result []T
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("failed to decode response from %s: %v", url, err)
	}
	return result
}

func fetchPullRequests(repo string, authors []string) []PullRequest {
	url := fmt.Sprintf("%s/repos/%s/pulls?state=all&per_page=100", baseUrl, repo)
	pullRequests := get[PullRequest](url)

	if authors == nil {
		return pullRequests
	}

	var userPullRequests []PullRequest
	for _, pr := range pullRequests {
		for _, author := range authors {
			if pr.User.Login == author {
				userPullRequests = append(userPullRequests, pr)
			}
		}
	}

	return userPullRequests
}

func fetchCommits(repo string, author string) []Commit {
	url := fmt.Sprintf("%s/repos/%s/commits?author=%s&per_page=100", baseUrl, repo, author)
	return get[Commit](url)
}

func fetchReviews(repo string, username string) []Review {
	var allReviews []Review
	prs := fetchPullRequests(repo, nil)

	for _, pr := range prs {
		url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews", baseUrl, repo, pr.Number)
		reviews := get[Review](url)

		for _, review := range reviews {
			if review.User.Login == username {
				allReviews = append(allReviews, review)
			}
		}
	}
	return allReviews
}

func fetchIssueComments(repo string, username string) []Comment {
	url := fmt.Sprintf("%s/repos/%s/issues/comments?per_page=100", baseUrl, repo)
	comments := get[Comment](url)

	var userComments []Comment
	for _, comment := range comments {
		if comment.User.Login == username {
			userComments = append(userComments, comment)
		}
	}
	return userComments
}

func fetchIssues(repo string, username string) []Issue {
	url := fmt.Sprintf("%s/repos/%s/issues?creator=%s&state=all&per_page=100", baseUrl, repo, username)
	return get[Issue](url)
}

func fetchPRFiles(repo string, prNumber int) []PRFile {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/files", baseUrl, repo, prNumber)
	return get[PRFile](url)
}

func analyzeRepo(repo string, username string, dateThreshold time.Time) Analytics {
	prs := fetchPullRequests(repo, []string{username})
	commits := fetchCommits(repo, username)
	reviews := fetchReviews(repo, username)
	comments := fetchIssueComments(repo, username)
	issues := fetchIssues(repo, username)

	analytics := Analytics{
		RepoName: repo,
		MonthlyBreakdown: make(map[string]MonthlyData),
		TechnologyBreakdown: make(map[string]int),
	}
	var durations []time.Duration

	// Analyze PRs and their files
	analytics.TotalPRs = len(prs)
	for _, pr := range prs {
		analytics.LinesAdded += pr.Additions
		analytics.LinesRemoved += pr.Deletions
		analytics.FilesChanged += pr.ChangedFiles

		// Monthly breakdown for PRs
		monthKey := pr.CreatedAt.Format("2006-01")
		if monthData, exists := analytics.MonthlyBreakdown[monthKey]; exists {
			monthData.PRs++
			analytics.MonthlyBreakdown[monthKey] = monthData
		} else {
			analytics.MonthlyBreakdown[monthKey] = MonthlyData{
				Month: monthKey,
				PRs:   1,
			}
		}

		// Analyze PR files for technology breakdown
		prFiles := fetchPRFiles(repo, pr.Number)
		for _, file := range prFiles {
			tech := analyzeTechnology(file.Filename)
			analytics.TechnologyBreakdown[tech] += file.Changes
			
			// Classify file types for code quality metrics
			isTest, isDoc, isConfig, isCI := classifyFile(file.Filename)
			if isTest {
				analytics.CodeQualityMetrics.TestFiles++
				analytics.CodeQualityMetrics.TestLinesAdded += file.Additions
			}
			if isDoc {
				analytics.CodeQualityMetrics.DocFiles++
				analytics.CodeQualityMetrics.DocLinesAdded += file.Additions
			}
			if isConfig {
				analytics.CodeQualityMetrics.ConfigFiles++
			}
			if isCI {
				analytics.CodeQualityMetrics.CIFiles++
			}
		}

		if pr.MergedAt != nil {
			analytics.MergedPRs++
			durations = append(durations, pr.MergedAt.Sub(pr.CreatedAt))

			if pr.MergedAt.After(dateThreshold) {
				analytics.RecentPRs++
			}
		}
	}

	// Calculate average merge time
	if len(durations) > 0 {
		var total time.Duration
		for _, d := range durations {
			total += d
		}
		analytics.AvgMergeTime = total / time.Duration(len(durations))
	}

	// Analyze commits
	analytics.TotalCommits = len(commits)
	for _, c := range commits {
		// Monthly breakdown for commits
		monthKey := c.Commit.Author.Date.Format("2006-01")
		if monthData, exists := analytics.MonthlyBreakdown[monthKey]; exists {
			monthData.Commits++
			monthData.LinesAdded += c.Stats.Additions
			monthData.LinesRemoved += c.Stats.Deletions
			analytics.MonthlyBreakdown[monthKey] = monthData
		} else {
			analytics.MonthlyBreakdown[monthKey] = MonthlyData{
				Month:        monthKey,
				Commits:      1,
				LinesAdded:   c.Stats.Additions,
				LinesRemoved: c.Stats.Deletions,
			}
		}

		// Work classification
		workType := classifyCommitWork(c.Commit.Message)
		switch workType {
		case "BugFix":
			analytics.WorkClassification.BugFixes++
		case "Feature":
			analytics.WorkClassification.Features++
		case "Refactoring":
			analytics.WorkClassification.Refactoring++
		case "Documentation":
			analytics.WorkClassification.Documentation++
		case "Infrastructure":
			analytics.WorkClassification.Infrastructure++
		}

		if c.Commit.Author.Date.After(dateThreshold) {
			analytics.RecentCommits++
		}
		// Add line changes from commits (if available)
		analytics.LinesAdded += c.Stats.Additions
		analytics.LinesRemoved += c.Stats.Deletions
	}

	// Analyze reviews
	analytics.CodeReviews = len(reviews)
	for _, review := range reviews {
		// Monthly breakdown for reviews
		monthKey := review.SubmittedAt.Format("2006-01")
		if monthData, exists := analytics.MonthlyBreakdown[monthKey]; exists {
			monthData.Reviews++
			analytics.MonthlyBreakdown[monthKey] = monthData
		} else {
			analytics.MonthlyBreakdown[monthKey] = MonthlyData{
				Month:   monthKey,
				Reviews: 1,
			}
		}

		if review.SubmittedAt.After(dateThreshold) {
			analytics.RecentReviews++
		}
	}

	// Analyze comments
	analytics.IssueComments = len(comments)
	for _, comment := range comments {
		// Monthly breakdown for comments
		monthKey := comment.CreatedAt.Format("2006-01")
		if monthData, exists := analytics.MonthlyBreakdown[monthKey]; exists {
			monthData.Comments++
			analytics.MonthlyBreakdown[monthKey] = monthData
		} else {
			analytics.MonthlyBreakdown[monthKey] = MonthlyData{
				Month:    monthKey,
				Comments: 1,
			}
		}

		if comment.CreatedAt.After(dateThreshold) {
			analytics.RecentComments++
		}
	}

	// Analyze issues
	analytics.IssuesCreated = len(issues)
	for _, issue := range issues {
		if issue.ClosedAt != nil {
			analytics.IssuesClosed++
		}
	}

	// Calculate derived metrics
	analytics.OpenPRs = analytics.TotalPRs - analytics.MergedPRs
	analytics.NetLinesChanged = analytics.LinesAdded - analytics.LinesRemoved

	if analytics.TotalPRs > 0 {
		analytics.PRSuccessRate = float64(analytics.MergedPRs) / float64(analytics.TotalPRs) * 100
	}

	daysSince := time.Since(dateThreshold).Hours() / 24
	weeksSince := daysSince / 7
	if weeksSince > 0 {
		analytics.CommitFrequency = float64(analytics.RecentCommits) / weeksSince
	}

	if analytics.NetLinesChanged != 0 {
		analytics.CodeChurnRate = float64(analytics.LinesAdded+analytics.LinesRemoved) / float64(abs(analytics.NetLinesChanged))
	}

	analytics.CollaborationScore = analytics.CodeReviews + analytics.IssueComments
	analytics.TotalActivity = getTotalActivityScore(analytics)

	// Enhanced activity check - includes any form of contribution
	analytics.HasActivity = analytics.TotalPRs > 0 ||
		analytics.TotalCommits > 0 ||
		analytics.CodeReviews > 0 ||
		analytics.IssueComments > 0 ||
		analytics.IssuesCreated > 0

	return analytics
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func sortAnalyticsByActivity(analytics []Analytics) []Analytics {
	// Create a copy to avoid modifying original slice
	sorted := make([]Analytics, len(analytics))
	copy(sorted, analytics)

	// Sort by total activity descending (highest activity first)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalActivity > sorted[j].TotalActivity
	})

	return sorted
}

func getTotalActivityScore(a Analytics) float64 {
	// Comprehensive activity scoring for sorting
	score := 0.0
	score += float64(a.TotalPRs) * 10     // PRs weighted heavily
	score += float64(a.TotalCommits) * 2  // Commits are frequent
	score += float64(a.CodeReviews) * 5   // Reviews show collaboration
	score += float64(a.IssueComments) * 3 // Comments show engagement
	score += float64(a.IssuesCreated) * 4 // Issue creation shows initiative
	score += float64(a.LinesAdded) * 0.01 // Code volume (small weight)
	return score
}

func generateDetailedReport(allAnalytics []Analytics, totalAnalytics Analytics, username string, dateRange string) {
	fmt.Printf("\nGITHUB ACTIVITY ANALYSIS\n")
	fmt.Printf("User: %s\n", username)
	fmt.Printf("Period: Last %s\n", dateRange)
	fmt.Printf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Summary metrics
	fmt.Println("SUMMARY METRICS")
	fmt.Println("================")
	fmt.Printf("Total Repositories Analyzed: %d\n", len(allAnalytics))
	fmt.Printf("Active Repositories: %d\n", countActiveRepos(allAnalytics))

	// Core metrics
	fmt.Println("CORE METRICS")
	fmt.Println("=============")
	fmt.Printf("Pull Requests: %d total, %d merged, %d open (%.1f%% success rate)\n",
		totalAnalytics.TotalPRs, totalAnalytics.MergedPRs, totalAnalytics.OpenPRs, totalAnalytics.PRSuccessRate)
	fmt.Printf("Commits: %d total, %d in period (%.1f commits/week)\n",
		totalAnalytics.TotalCommits, totalAnalytics.RecentCommits, totalAnalytics.CommitFrequency)
	fmt.Printf("Code Impact: +%d/-%d lines, net %d lines, %d files\n",
		totalAnalytics.LinesAdded, totalAnalytics.LinesRemoved, totalAnalytics.NetLinesChanged, totalAnalytics.FilesChanged)
	fmt.Printf("Collaboration: %d reviews, %d comments (score: %d)\n",
		totalAnalytics.CodeReviews, totalAnalytics.IssueComments, totalAnalytics.CollaborationScore)

	if totalAnalytics.AvgMergeTime > 0 {
		fmt.Printf("Avg Merge Time: %s\n", totalAnalytics.AvgMergeTime.Round(time.Hour))
	}

	if totalAnalytics.CodeChurnRate > 0 {
		fmt.Printf("Code Churn Rate: %.2f\n", totalAnalytics.CodeChurnRate)
	}

	// Technology breakdown
	fmt.Println("\nTECHNOLOGY BREAKDOWN")
	fmt.Println("===================")
	if len(totalAnalytics.TechnologyBreakdown) > 0 {
		for tech, changes := range totalAnalytics.TechnologyBreakdown {
			if changes > 0 {
				fmt.Printf("%-15s: %d changes\n", tech, changes)
			}
		}
	}

	// Work classification
	fmt.Println("\nWORK CLASSIFICATION")
	fmt.Println("==================")
	total := totalAnalytics.WorkClassification
	fmt.Printf("Features:       %d commits\n", total.Features)
	fmt.Printf("Bug Fixes:      %d commits\n", total.BugFixes)
	fmt.Printf("Refactoring:    %d commits\n", total.Refactoring)
	fmt.Printf("Documentation:  %d commits\n", total.Documentation)
	fmt.Printf("Infrastructure: %d commits\n", total.Infrastructure)

	// Code quality metrics
	fmt.Println("\nCODE QUALITY CONTRIBUTIONS")
	fmt.Println("==========================")
	quality := totalAnalytics.CodeQualityMetrics
	fmt.Printf("Test files:         %d files, +%d lines\n", quality.TestFiles, quality.TestLinesAdded)
	fmt.Printf("Documentation:      %d files, +%d lines\n", quality.DocFiles, quality.DocLinesAdded)
	fmt.Printf("Configuration:      %d files\n", quality.ConfigFiles)
	fmt.Printf("CI/CD:              %d files\n", quality.CIFiles)

	// Repository breakdown (sorted by activity)
	fmt.Println("\nREPOSITORY BREAKDOWN (sorted by activity)")
	fmt.Println("==========================================")

	// Sort repositories by activity level
	sortedAnalytics := sortAnalyticsByActivity(allAnalytics)

	for i, analytics := range sortedAnalytics {
		if analytics.HasActivity {
			fmt.Printf("\n%d. %s:\n", i+1, analytics.RepoName)
			fmt.Printf("   PRs: %d (%d merged) | Commits: %d | Reviews: %d | Comments: %d\n",
				analytics.TotalPRs, analytics.MergedPRs, analytics.RecentCommits,
				analytics.RecentReviews, analytics.RecentComments)
			fmt.Printf("   Code: +%d/-%d lines, %d files | Frequency: %.1f commits/week\n",
				analytics.LinesAdded, analytics.LinesRemoved, analytics.FilesChanged, analytics.CommitFrequency)
			
			// Show top technologies for this repo
			if len(analytics.TechnologyBreakdown) > 0 {
				fmt.Printf("   Technologies: ")
				count := 0
				for tech, changes := range analytics.TechnologyBreakdown {
					if count < 3 && changes > 0 {
						fmt.Printf("%s (%d) ", tech, changes)
						count++
					}
				}
				fmt.Println()
			}
		}
	}

	// Performance insights
	fmt.Println("\nINSIGHTS")
	fmt.Println("=========")
	generateInsights(totalAnalytics, allAnalytics)
}

func countActiveRepos(analytics []Analytics) int {
	count := 0
	for _, a := range analytics {
		if a.HasActivity {
			count++
		}
	}
	return count
}

func generateInsights(total Analytics, all []Analytics) {
	if total.CommitFrequency > 3 {
		fmt.Println("* High commit frequency indicates consistent contribution")
	}

	if total.PRSuccessRate > 80 {
		fmt.Println("* Excellent PR success rate shows quality contributions")
	}

	if total.CollaborationScore > 50 {
		fmt.Println("* Strong collaboration through reviews and discussions")
	}

	if total.NetLinesChanged > 1000 {
		fmt.Println("* Significant code impact with substantial additions")
	}

	active := countActiveRepos(all)
	if active > len(all)/2 {
		fmt.Println("* Active across majority of repositories")
	}

	if total.CodeChurnRate > 0 && total.CodeChurnRate < 2 {
		fmt.Println("* Efficient code changes with low churn rate")
	}
}

func generateGraphData(analytics []Analytics, username string) error {
	// Sort analytics by activity before exporting
	sortedAnalytics := sortAnalyticsByActivity(analytics)

	// Generate CSV for graphing tools
	filename := fmt.Sprintf("activity_graph_data_%s_%s.csv", username, time.Now().Format("2006-01-02"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Rank", "Repository", "Commits", "PRs", "Reviews", "Comments", "LinesAdded", "LinesRemoved", "CommitFrequency", "PRSuccessRate"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data for each repo with activity (sorted by activity)
	rank := 1
	for _, a := range sortedAnalytics {
		if a.HasActivity {
			row := []string{
				strconv.Itoa(rank),
				a.RepoName,
				strconv.Itoa(a.RecentCommits),
				strconv.Itoa(a.TotalPRs),
				strconv.Itoa(a.CodeReviews),
				strconv.Itoa(a.IssueComments),
				strconv.Itoa(a.LinesAdded),
				strconv.Itoa(a.LinesRemoved),
				fmt.Sprintf("%.2f", a.CommitFrequency),
				fmt.Sprintf("%.1f", a.PRSuccessRate),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
			rank++
		}
	}

	fmt.Printf("Graph data exported to: %s (sorted by activity)\n", filename)
	return nil
}

func generateTimelineData(allAnalytics []Analytics, username string) error {
	// Aggregate monthly data across all repositories
	monthlyTotals := make(map[string]MonthlyData)
	
	for _, analytics := range allAnalytics {
		for monthKey, monthData := range analytics.MonthlyBreakdown {
			if existing, exists := monthlyTotals[monthKey]; exists {
				existing.Commits += monthData.Commits
				existing.PRs += monthData.PRs
				existing.Reviews += monthData.Reviews
				existing.Comments += monthData.Comments
				existing.LinesAdded += monthData.LinesAdded
				existing.LinesRemoved += monthData.LinesRemoved
				monthlyTotals[monthKey] = existing
			} else {
				monthlyTotals[monthKey] = monthData
			}
		}
	}
	
	// Generate timeline CSV
	filename := fmt.Sprintf("timeline_data_%s_%s.csv", username, time.Now().Format("2006-01-02"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Month", "Commits", "PRs", "Reviews", "Comments", "LinesAdded", "LinesRemoved", "NetLines"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Sort months chronologically
	var months []string
	for month := range monthlyTotals {
		months = append(months, month)
	}
	sort.Strings(months)

	// Write data
	for _, month := range months {
		data := monthlyTotals[month]
		netLines := data.LinesAdded - data.LinesRemoved
		row := []string{
			month,
			strconv.Itoa(data.Commits),
			strconv.Itoa(data.PRs),
			strconv.Itoa(data.Reviews),
			strconv.Itoa(data.Comments),
			strconv.Itoa(data.LinesAdded),
			strconv.Itoa(data.LinesRemoved),
			strconv.Itoa(netLines),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("Timeline data exported to: %s\n", filename)
	return nil
}

func exportToCSV(analytics Analytics, username string, dateRange string) error {
	filename := fmt.Sprintf("github_analysis_%s_%s_%s.csv", username, dateRange, time.Now().Format("2006-01-02"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Metric", "Value", "Period"}
	if err := writer.Write(header); err != nil {
		return err
	}

	period := fmt.Sprintf("Last %s", dateRange)
	data := [][]string{
		{"Total PRs", strconv.Itoa(analytics.TotalPRs), "All time"},
		{"Merged PRs", strconv.Itoa(analytics.MergedPRs), "All time"},
		{"Open PRs", strconv.Itoa(analytics.OpenPRs), "All time"},
		{"PR Success Rate", fmt.Sprintf("%.1f%%", analytics.PRSuccessRate), "Calculated"},
		{"Recent PRs", strconv.Itoa(analytics.RecentPRs), period},
		{"Total Commits", strconv.Itoa(analytics.TotalCommits), "All time"},
		{"Recent Commits", strconv.Itoa(analytics.RecentCommits), period},
		{"Commit Frequency", fmt.Sprintf("%.2f", analytics.CommitFrequency), "Per week"},
		{"Code Reviews", strconv.Itoa(analytics.CodeReviews), "All time"},
		{"Recent Reviews", strconv.Itoa(analytics.RecentReviews), period},
		{"Issue Comments", strconv.Itoa(analytics.IssueComments), "All time"},
		{"Recent Comments", strconv.Itoa(analytics.RecentComments), period},
		{"Lines Added", strconv.Itoa(analytics.LinesAdded), "All time"},
		{"Lines Removed", strconv.Itoa(analytics.LinesRemoved), "All time"},
		{"Net Lines Changed", strconv.Itoa(analytics.NetLinesChanged), "All time"},
		{"Files Changed", strconv.Itoa(analytics.FilesChanged), "All time"},
		{"Code Churn Rate", fmt.Sprintf("%.2f", analytics.CodeChurnRate), "Calculated"},
		{"Collaboration Score", strconv.Itoa(analytics.CollaborationScore), "Calculated"},
		{"Issues Created", strconv.Itoa(analytics.IssuesCreated), "All time"},
		{"Issues Closed", strconv.Itoa(analytics.IssuesClosed), "All time"},
	}

	if analytics.AvgMergeTime > 0 {
		data = append(data, []string{"Avg Merge Time", analytics.AvgMergeTime.String(), "All time"})
	}

	for _, row := range data {
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	fmt.Printf("Data exported to: %s\n", filename)
	return nil
}

func exportToJSON(analytics Analytics, username string, dateRange string) error {
	type ExportData struct {
		Username    string    `json:"username"`
		GeneratedAt time.Time `json:"generated_at"`
		Analytics   Analytics `json:"analytics"`
		ImpactScore float64   `json:"impact_score"`
	}

	data := ExportData{
		Username:    username,
		GeneratedAt: time.Now(),
		Analytics:   analytics,
		ImpactScore: analytics.TotalActivity,
	}

	filename := fmt.Sprintf("github_analysis_%s_%s_%s.json", username, dateRange, time.Now().Format("2006-01-02"))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return err
	}

	fmt.Printf("Data exported to: %s\n", filename)
	return nil
}

func generateDashboardData(allAnalytics []Analytics, totalAnalytics Analytics, username string, dateRange string) error {
	type DashboardData struct {
		Username     string      `json:"username"`
		GeneratedAt  time.Time   `json:"generated_at"`
		Analytics    Analytics   `json:"analytics"`
		Repositories []Analytics `json:"repositories"`
	}

	data := DashboardData{
		Username:     username,
		GeneratedAt:  time.Now(),
		Analytics:    totalAnalytics,
		Repositories: allAnalytics,
	}

	// Export for dashboard
	file, err := os.Create("analytics_data.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return err
	}

	fmt.Printf("Dashboard data exported to: analytics_data.json\n")
	return nil
}

func generateHTMLDashboard(username string) error {
	// Read the dashboard template
	dashboardHTML, err := os.ReadFile("dashboard.html")
	if err != nil {
		// If template doesn't exist, create a basic one
		dashboardHTML = []byte(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GitHub Analytics Dashboard</title>
    <link rel="stylesheet" href="styles.css">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="dashboard-container">
        <h1>GitHub Analytics Dashboard</h1>
        <p>Open the browser console and check for analytics_data.json</p>
    </div>
    <script src="dashboard.js"></script>
</body>
</html>`)
	}

	// Generate the final HTML file
	filename := fmt.Sprintf("github_dashboard_%s_%s.html", username, time.Now().Format("2006-01-02"))
	err = os.WriteFile(filename, dashboardHTML, 0644)
	if err != nil {
		return err
	}

	fmt.Printf("HTML Dashboard generated: %s\n", filename)
	fmt.Printf("Open %s in your browser to view the interactive dashboard\n", filename)
	return nil
}

func main() {
	// Parse command line flags
	var (
		username  = flag.String("user", "", "GitHub username (required)")
		org       = flag.String("org", "", "GitHub organization (optional, mutually exclusive with -repos)")
		repos     = flag.String("repos", "", "Comma-separated list of repos in format owner/repo (optional, mutually exclusive with -org)")
		dateRange = flag.String("date", "3m", "Date range to analyze (format: 30d, 3m, 1y)")
	)
	flag.Parse()

	if *username == "" {
		log.Fatal("Username is required. Use -user flag.")
	}

	if *org == "" && *repos == "" {
		log.Fatal("Either organization (-org) or repos (-repos) must be specified.")
	}

	if *org != "" && *repos != "" {
		log.Fatal("Cannot specify both organization and repos. Use either -org or -repos.")
	}

	dateThreshold, err := parseDuration(*dateRange)
	if err != nil {
		log.Fatalf("Invalid date range: %v", err)
	}

	// Determine which repos to analyze
	var reposToAnalyze []string
	if *org != "" {
		reposToAnalyze = fetchOrgRepos(*org)
	} else {
		reposToAnalyze = strings.Split(*repos, ",")
		for i, repo := range reposToAnalyze {
			reposToAnalyze[i] = strings.TrimSpace(repo)
		}
	}

	fmt.Printf("Analyzing %d repositories...\n", len(reposToAnalyze))

	var totalAnalytics Analytics
	var allAnalytics []Analytics
	var skippedRepos []string

	for i, repo := range reposToAnalyze {
		fmt.Printf("[%d/%d] %s", i+1, len(reposToAnalyze), repo)
		analytics := analyzeRepo(repo, *username, dateThreshold)

		// Smart activity check - only include repos with any form of contribution
		if analytics.HasActivity {
			allAnalytics = append(allAnalytics, analytics)
			fmt.Printf(" - ACTIVE\n")

			// Aggregate totals
			totalAnalytics.TotalPRs += analytics.TotalPRs
			totalAnalytics.MergedPRs += analytics.MergedPRs
			totalAnalytics.RecentPRs += analytics.RecentPRs
			totalAnalytics.TotalCommits += analytics.TotalCommits
			totalAnalytics.RecentCommits += analytics.RecentCommits
			totalAnalytics.CodeReviews += analytics.CodeReviews
			totalAnalytics.RecentReviews += analytics.RecentReviews
			totalAnalytics.IssueComments += analytics.IssueComments
			totalAnalytics.RecentComments += analytics.RecentComments
			totalAnalytics.LinesAdded += analytics.LinesAdded
			totalAnalytics.LinesRemoved += analytics.LinesRemoved
			totalAnalytics.FilesChanged += analytics.FilesChanged
			totalAnalytics.IssuesCreated += analytics.IssuesCreated
			totalAnalytics.IssuesClosed += analytics.IssuesClosed
			
			// Aggregate enhanced metrics
			if totalAnalytics.TechnologyBreakdown == nil {
				totalAnalytics.TechnologyBreakdown = make(map[string]int)
			}
			for tech, count := range analytics.TechnologyBreakdown {
				totalAnalytics.TechnologyBreakdown[tech] += count
			}
			
			// Aggregate work classification
			totalAnalytics.WorkClassification.Features += analytics.WorkClassification.Features
			totalAnalytics.WorkClassification.BugFixes += analytics.WorkClassification.BugFixes
			totalAnalytics.WorkClassification.Refactoring += analytics.WorkClassification.Refactoring
			totalAnalytics.WorkClassification.Documentation += analytics.WorkClassification.Documentation
			totalAnalytics.WorkClassification.Infrastructure += analytics.WorkClassification.Infrastructure
			
			// Aggregate code quality metrics
			totalAnalytics.CodeQualityMetrics.TestFiles += analytics.CodeQualityMetrics.TestFiles
			totalAnalytics.CodeQualityMetrics.TestLinesAdded += analytics.CodeQualityMetrics.TestLinesAdded
			totalAnalytics.CodeQualityMetrics.DocFiles += analytics.CodeQualityMetrics.DocFiles
			totalAnalytics.CodeQualityMetrics.DocLinesAdded += analytics.CodeQualityMetrics.DocLinesAdded
			totalAnalytics.CodeQualityMetrics.ConfigFiles += analytics.CodeQualityMetrics.ConfigFiles
			totalAnalytics.CodeQualityMetrics.CIFiles += analytics.CodeQualityMetrics.CIFiles
		} else {
			skippedRepos = append(skippedRepos, repo)
			fmt.Printf(" - NO ACTIVITY (skipped)\n")
		}
	}

	if len(allAnalytics) == 0 {
		fmt.Printf("\nNo activity found for user %s in the specified repositories/organization.\n", *username)
		if len(skippedRepos) > 0 {
			fmt.Printf("Repositories with no activity (%d): %s\n", len(skippedRepos), strings.Join(skippedRepos, ", "))
		}
		return
	}

	// Show summary of filtering
	fmt.Printf("\nFiltering complete:")
	fmt.Printf("\n  Active repositories: %d", len(allAnalytics))
	fmt.Printf("\n  Skipped (no activity): %d", len(skippedRepos))
	if len(skippedRepos) > 0 && len(skippedRepos) <= 10 {
		fmt.Printf("\n  Skipped repos: %s", strings.Join(skippedRepos, ", "))
	} else if len(skippedRepos) > 10 {
		fmt.Printf("\n  Skipped repos: %s... and %d more", strings.Join(skippedRepos[:10], ", "), len(skippedRepos)-10)
	}
	fmt.Println()

	// Calculate aggregated metrics
	var allDurations []time.Duration
	for _, analytics := range allAnalytics {
		if analytics.AvgMergeTime > 0 {
			allDurations = append(allDurations, analytics.AvgMergeTime)
		}
	}
	if len(allDurations) > 0 {
		var total time.Duration
		for _, d := range allDurations {
			total += d
		}
		totalAnalytics.AvgMergeTime = total / time.Duration(len(allDurations))
	}

	// Recalculate derived metrics for totals
	totalAnalytics.OpenPRs = totalAnalytics.TotalPRs - totalAnalytics.MergedPRs
	totalAnalytics.NetLinesChanged = totalAnalytics.LinesAdded - totalAnalytics.LinesRemoved

	if totalAnalytics.TotalPRs > 0 {
		totalAnalytics.PRSuccessRate = float64(totalAnalytics.MergedPRs) / float64(totalAnalytics.TotalPRs) * 100
	}

	daysSince := time.Since(dateThreshold).Hours() / 24
	weeksSince := daysSince / 7
	if weeksSince > 0 {
		totalAnalytics.CommitFrequency = float64(totalAnalytics.RecentCommits) / weeksSince
	}

	if totalAnalytics.NetLinesChanged != 0 {
		totalAnalytics.CodeChurnRate = float64(totalAnalytics.LinesAdded+totalAnalytics.LinesRemoved) / float64(abs(totalAnalytics.NetLinesChanged))
	}

	totalAnalytics.CollaborationScore = totalAnalytics.CodeReviews + totalAnalytics.IssueComments
	totalAnalytics.TotalActivity = getTotalActivityScore(totalAnalytics)
	totalAnalytics.HasActivity = len(allAnalytics) > 0

	generateDetailedReport(allAnalytics, totalAnalytics, *username, *dateRange)

	// Export data
	if err := exportToJSON(totalAnalytics, *username, *dateRange); err != nil {
		log.Printf("Failed to export JSON: %v", err)
	}

	if err := exportToCSV(totalAnalytics, *username, *dateRange); err != nil {
		log.Printf("Failed to export CSV: %v", err)
	}

	if err := generateGraphData(allAnalytics, *username); err != nil {
		log.Printf("Failed to export graph data: %v", err)
	}

	if err := generateTimelineData(allAnalytics, *username); err != nil {
		log.Printf("Failed to export timeline data: %v", err)
	}

	if err := generateDashboardData(allAnalytics, totalAnalytics, *username, *dateRange); err != nil {
		log.Printf("Failed to export dashboard data: %v", err)
	}

	if err := generateHTMLDashboard(*username); err != nil {
		log.Printf("Failed to generate HTML dashboard: %v", err)
	}

	fmt.Printf("\nAnalysis complete. Found activity in %d repositories.\n", len(allAnalytics))
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "GitHub Activity Analyzer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nDate format examples:\n")
		fmt.Fprintf(os.Stderr, "  30d  = last 30 days\n")
		fmt.Fprintf(os.Stderr, "  3m   = last 3 months\n")
		fmt.Fprintf(os.Stderr, "  1y   = last 1 year\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -user johndoe -org mycompany\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -user johndoe -repos \"owner/repo1,owner/repo2\" -date 30d\n", os.Args[0])
	}
}
