# What this workflow does

Trigger: Runs on PR events — when a PR is opened, edited, updated, or reopened.

Scope: Uses pull_request_target so it can label PRs coming from forks (safe since it only reads the PR body and applies labels).

Regex matching: Scans the PR description (body) for your checkboxes:
- [ ] **This is a bugfix**
- [ ] **Include in next stabilisation release**

Behavior:
If a checkbox line exists:
[x] → adds the label.
[ ] → removes the label (if already applied).

If a checkbox line is missing → leaves the label as is.

Automatic label creation: If `bugfix` or `include-in-next-release` labels don’t exist in the repo, the Action will create them automatically with readable colors and descriptions.
