# GitHub Metrics

This project generates GitHub user metrics and displays them in an HTML table. The metrics include commits, hits of code (HoC), issues, lifecycle of pull requests (LcP), messages in pull requests, pull requests created and merged, reviews of pull requests, and an overall score.

## Features

- Collects various metrics from GitHub repositories.
- Filters repositories by organization.
- Generates an HTML file to display the metrics in a table.
- Provides links to detailed GitHub searches for each metric.

## Metrics Explained

- **Commits**: Total number of non-merge Git commits to the default branch, authored by the user.
- **HoC**: Total number of user's hits of code.
- **Issues**: Total number of issues submitted by the user.
- **LcP**: Average lifecycle of a pull request in hours.
- **Msgs**: Total number of messages posted in pull requests where the user was a reviewer.
- **Pulls**: Total number of pull requests created by the user and already merged.
- **Reviews**: Total number of merged pull requests that were reviewed by the user.
- **Score**: Arithmetic summary of all metrics with multipliers:
  - 1×HoC
  - 250×Pulls
  - 50×Issues
  - 5×Commits
  - 150×Reviews
  - 5×Msgs

## Setup

1. Clone the repository:
    ```sh
    git clone https://github.com/yourusername/github-metrics.git
    cd github-metrics
    ```

2. Install dependencies:
    ```sh
    go get 
    ```

3. Create a `.githubmetrics` file with your GitHub token and other configurations:
    ```
    --token=YOUR_GITHUB_TOKEN
    --days=30
    --verbose=true
    --metric=all
    --delay=30
    --organization=yourorganization

    --coder=yourusername1
    --coder=yourusername2
    --coder=yourusername3

    ```

4. Run the application:
    ```sh
    go run main.go
    ```

## HTML Output

The generated HTML file will contain a table with the following columns:

- **User**
- **Commits**
- **HoC**
- **Issues**
- **LcP**
- **Msgs**
- **Pulls**
- **Reviews**
- **Score**

Each metric value in the table is a link to a detailed GitHub search for that specific metric.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
