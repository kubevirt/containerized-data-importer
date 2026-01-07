package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-github/v57/github"
)

type ReleaseData struct {
	org          string
	repo         string
	releaseRef   string
	prevRef      string
	spanRef      string
	githubClient *github.Client
	gitRepo      *git.Repository
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s [RELEASE_REF] [PREV_RELEASE_REF]\n", os.Args[0])
		os.Exit(1)
	}

	releaseData := &ReleaseData{
		org:        "kubevirt",
		repo:       "containerized-data-importer",
		releaseRef: os.Args[1],
		prevRef:    os.Args[2],
		spanRef:    fmt.Sprintf("%s..%s", os.Args[2], os.Args[1]),
	}

	// Initialize GitHub client with authentication if available
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		releaseData.githubClient = github.NewClient(nil).WithAuthToken(token)
		log.Printf("Using GitHub token for authentication")
	} else {
		releaseData.githubClient = github.NewClient(nil)
		log.Printf("Warning: No GITHUB_TOKEN set, using unauthenticated requests (rate limited)")
	}

	// Open git repository (current directory)
	var err error
	releaseData.gitRepo, err = git.PlainOpen("../..")
	if err != nil {
		log.Fatalf("Failed to open git repository: %v", err)
	}

	fmt.Printf("Release span: %s\n", releaseData.spanRef)

	announcement := releaseData.generateReleaseAnnouncement()

	// Write to file in project root
	err = os.WriteFile("../../release-announcement", []byte(announcement), 0644)
	if err != nil {
		log.Fatalf("Failed to write release announcement: %v", err)
	}

	fmt.Print(announcement)
}

func (r *ReleaseData) generateReleaseAnnouncement() string {
	var announcement strings.Builder

	// Generate summary
	summary := r.generateSummary()
	announcement.WriteString(summary + "\n\n")

	// Generate download links
	downloads := r.generateDownloads()
	announcement.WriteString(downloads + "\n\n\n")

	// Notable changes section
	announcement.WriteString(r.underline("-", "Notable changes") + "\n\n")

	releaseNotes, otherChanges := r.generateReleaseNotes()
	announcement.WriteString(releaseNotes + "\n\n")

	// Other changes section (if there are any)
	if otherChanges != "" {
		announcement.WriteString("\n" + r.underline("-", "Other changes") + "\n\n")
		announcement.WriteString(otherChanges + "\n\n")
	}

	// Contributors section
	announcement.WriteString(r.underline("-", "Contributors") + "\n\n")
	contributors := r.generateContributors()
	announcement.WriteString(contributors + "\n\n")

	// Additional resources
	announcement.WriteString("Additional Resources\n")
	announcement.WriteString(strings.Repeat("-", 20) + "\n")
	announcement.WriteString("- Mailing list: <https://groups.google.com/forum/#!forum/kubevirt-dev>\n")
	announcement.WriteString("- [How to contribute][contributing]\n")
	announcement.WriteString("- [License][license]\n\n")
	announcement.WriteString("[contributing]: https://github.com/kubevirt/containerized-data-importer/blob/main/hack/README.md\n")
	announcement.WriteString("[license]: https://github.com/kubevirt/containerized-data-importer/blob/main/LICENSE\n")

	return announcement.String()
}

func (r *ReleaseData) underline(char string, text string) string {
	return text + "\n" + strings.Repeat(char, len(text))
}

func (r *ReleaseData) generateSummary() string {
	commitCount := r.getCommitCount()
	contributorCount := r.getContributorCount()
	diffStats := r.getDiffStats()

	return fmt.Sprintf("This release follows %s and consists of %d changes, contributed by\n%d people, leading to %s.",
		r.prevRef, commitCount, contributorCount, diffStats)
}

func (r *ReleaseData) generateDownloads() string {
	ghRelURL := fmt.Sprintf("https://github.com/kubevirt/containerized-data-importer/releases/tag/%s", r.releaseRef)

	return fmt.Sprintf(`The source code and selected binaries are available for download at:
<%s>.

Pre-built CDI containers are published on Quay.io and can be viewed at:
<https://quay.io/repository/kubevirt/cdi-controller/>
<https://quay.io/repository/kubevirt/cdi-importer/>
<https://quay.io/repository/kubevirt/cdi-cloner/>
<https://quay.io/repository/kubevirt/cdi-uploadproxy/>
<https://quay.io/repository/kubevirt/cdi-apiserver/>
<https://quay.io/repository/kubevirt/cdi-uploadserver/>
<https://quay.io/repository/kubevirt/cdi-operator/>`, ghRelURL)
}

func (r *ReleaseData) generateReleaseNotes() (string, string) {
	commits, err := r.getCommitsInRange()
	if err != nil {
		log.Printf("Warning: Failed to get commits: %v", err)
		return "Release notes could not be automatically generated.", ""
	}

	prNumbers := r.extractPRNumbers(commits)
	log.Printf("Debug: Found PR numbers: %v", prNumbers)

	if len(prNumbers) == 0 {
		return "No pull requests found in commit history for this release.", ""
	}

	var notableChanges []string
	var otherChanges []string

	for _, prNum := range prNumbers {
		note, hasReleaseNote := r.getReleaseNote(prNum)
		if note != "" {
			if hasReleaseNote {
				notableChanges = append(notableChanges, note)
			} else {
				otherChanges = append(otherChanges, note)
			}
		}
	}

	notableSection := ""
	if len(notableChanges) > 0 {
		notableSection = strings.Join(notableChanges, "\n")
	} else {
		notableSection = "No significant changes with detailed release notes found."
	}

	otherSection := ""
	if len(otherChanges) > 0 {
		otherSection = strings.Join(otherChanges, "\n")
	}

	return notableSection, otherSection
}

func (r *ReleaseData) generateContributors() string {
	contributorCount := r.getContributorCount()
	contributorList := r.getContributorList()

	return fmt.Sprintf("%d people contributed to this release:\n\n%s", contributorCount, contributorList)
}

func (r *ReleaseData) getCommitsInRange() ([]*object.Commit, error) {
	// Get the commit objects for the range
	prevHash, err := r.gitRepo.ResolveRevision(plumbing.Revision(r.prevRef))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve prev ref %s: %v", r.prevRef, err)
	}

	releaseHash, err := r.gitRepo.ResolveRevision(plumbing.Revision(r.releaseRef))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve release ref %s: %v", r.releaseRef, err)
	}

	// Get all commits reachable from release ref
	releaseCommitIter, err := r.gitRepo.Log(&git.LogOptions{From: *releaseHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get release commit iterator: %v", err)
	}
	defer releaseCommitIter.Close()

	// Build set of all commits reachable from release ref
	releaseCommits := make(map[plumbing.Hash]*object.Commit)
	err = releaseCommitIter.ForEach(func(c *object.Commit) error {
		releaseCommits[c.Hash] = c
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate release commits: %v", err)
	}

	// Get all commits reachable from prev ref
	prevCommitIter, err := r.gitRepo.Log(&git.LogOptions{From: *prevHash})
	if err != nil {
		return nil, fmt.Errorf("failed to get prev commit iterator: %v", err)
	}
	defer prevCommitIter.Close()

	// Remove commits that are also reachable from prev ref
	err = prevCommitIter.ForEach(func(c *object.Commit) error {
		delete(releaseCommits, c.Hash)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate prev commits: %v", err)
	}

	// Convert back to slice
	var rangeCommits []*object.Commit
	for _, commit := range releaseCommits {
		rangeCommits = append(rangeCommits, commit)
	}

	// Sort commits by committer date (newest first)
	sort.Slice(rangeCommits, func(i, j int) bool {
		return rangeCommits[i].Committer.When.After(rangeCommits[j].Committer.When)
	})

	log.Printf("Debug: Found %d commits in range %s", len(rangeCommits), r.spanRef)
	return rangeCommits, nil
}

func (r *ReleaseData) extractPRNumbers(commits []*object.Commit) []int {
	// Try multiple regex patterns to catch different merge commit formats
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`Merge pull request #(\d+)`), // GitHub merge
		regexp.MustCompile(`\(#(\d+)\)`),                // Squash merge format
		regexp.MustCompile(`#(\d+)`),                    // Any mention of PR number
	}

	var prNumbers []int
	seen := make(map[int]bool)

	for _, commit := range commits {
		message := commit.Message
		if strings.TrimSpace(message) == "" {
			continue
		}

		for _, pattern := range patterns {
			matches := pattern.FindAllStringSubmatch(message, -1)
			for _, match := range matches {
				if len(match) > 1 {
					prNum, err := strconv.Atoi(match[1])
					if err == nil && !seen[prNum] {
						prNumbers = append(prNumbers, prNum)
						seen[prNum] = true
						break // Found a PR number, move to next commit
					}
				}
			}
		}
	}

	return prNumbers
}

func (r *ReleaseData) getReleaseNote(prNumber int) (string, bool) {
	log.Printf("Searching for release note for PR #%d", prNumber)

	pr, resp, err := r.githubClient.PullRequests.Get(context.Background(), r.org, r.repo, prNumber)
	if err != nil {
		// Handle 404 (PR not found) gracefully, but fail on other errors like rate limiting
		if resp != nil && resp.StatusCode == 404 {
			log.Printf("Warning: PR #%d not found (404), skipping", prNumber)
			return "", false
		}
		log.Fatalf("Failed to get PR #%d: %v", prNumber, err)
	}

	// Check for release-note-none label - these should go to "Other changes"
	hasReleaseNoneLabel := false
	for _, label := range pr.Labels {
		if label.Name != nil && *label.Name == "release-note-none" {
			hasReleaseNoneLabel = true
			break
		}
	}

	// If it has release-note-none, skip release note parsing and go straight to title
	if hasReleaseNoneLabel {
		if pr.Title != nil && *pr.Title != "" {
			return fmt.Sprintf("- [PR #%d] %s", prNumber, *pr.Title), false
		}
		return fmt.Sprintf("- [PR #%d] Changes merged", prNumber), false
	}

	// Extract release note from body - matches reference implementation
	if pr.Body != nil && *pr.Body != "" {
		body := strings.Split(*pr.Body, "\n")

		for i, line := range body {
			if strings.Contains(line, "```release-note") {
				// Collect all lines until closing ```
				var noteLines []string
				for j := i + 1; j < len(body); j++ {
					if strings.TrimSpace(body[j]) == "```" {
						break // Found closing ```
					}
					line := strings.TrimSpace(body[j])
					if line != "" { // Skip empty lines
						noteLines = append(noteLines, line)
					}
				}

				if len(noteLines) > 0 {
					// Join all lines with spaces
					note := strings.Join(noteLines, " ")
					// Clean up the note - matches reference implementation
					note = strings.ReplaceAll(note, "\r\n", "")
					note = strings.ReplaceAll(note, "\r", "")
					note = strings.TrimPrefix(note, "- ")
					note = strings.TrimPrefix(note, "-")
					// Check if it's "NONE" - matches reference implementation
					if !strings.Contains(strings.ToUpper(note), "NONE") && note != "" {
						return fmt.Sprintf("- [PR #%d] %s", prNumber, note), true
					}
				}
				break
			}
		}
	}

	// Fallback to title
	if pr.Title != nil && *pr.Title != "" {
		return fmt.Sprintf("- [PR #%d] %s", prNumber, *pr.Title), false
	}

	return fmt.Sprintf("- [PR #%d] Changes merged", prNumber), false
}

func (r *ReleaseData) getCommitCount() int {
	commits, err := r.getCommitsInRange()
	if err != nil {
		log.Printf("Warning: Failed to get commit count: %v", err)
		return 0
	}
	return len(commits)
}

func (r *ReleaseData) getContributorCount() int {
	commits, err := r.getCommitsInRange()
	if err != nil {
		log.Printf("Warning: Failed to get contributor count: %v", err)
		return 0
	}

	contributors := make(map[string]bool)
	for _, commit := range commits {
		if commit.Author.Email != "" {
			contributors[commit.Author.Email] = true
		}
	}

	return len(contributors)
}

func (r *ReleaseData) getContributorList() string {
	commits, err := r.getCommitsInRange()
	if err != nil {
		log.Printf("Warning: Failed to get contributor list: %v", err)
		return ""
	}

	contributorCounts := make(map[string]int)
	for _, commit := range commits {
		contributor := fmt.Sprintf("%s <%s>", commit.Author.Name, commit.Author.Email)
		contributorCounts[contributor]++
	}

	// Extract contributors and sort alphabetically
	var contributors []string
	for contributor := range contributorCounts {
		contributors = append(contributors, contributor)
	}
	sort.Strings(contributors)

	// Format with counts in alphabetical order
	var formatted []string
	for _, contributor := range contributors {
		count := contributorCounts[contributor]
		formatted = append(formatted, fmt.Sprintf("    %d\t%s", count, contributor))
	}

	return strings.Join(formatted, "\n")
}

func (r *ReleaseData) getDiffStats() string {
	commits, err := r.getCommitsInRange()
	if err != nil {
		log.Printf("Warning: Failed to get diff stats: %v", err)
		return "changes"
	}

	if len(commits) == 0 {
		return "no changes"
	}

	// Get the first and last commit for stats
	firstCommit := commits[len(commits)-1] // Oldest commit in range
	lastCommit := commits[0]               // Newest commit in range

	firstTree, err := firstCommit.Tree()
	if err != nil {
		return "changes"
	}

	lastTree, err := lastCommit.Tree()
	if err != nil {
		return "changes"
	}

	changes, err := firstTree.Diff(lastTree)
	if err != nil {
		return "changes"
	}

	filesChanged := len(changes)
	return fmt.Sprintf("%d files changed", filesChanged)
}
