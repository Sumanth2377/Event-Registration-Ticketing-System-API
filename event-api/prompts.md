# AI Prompts Used

As required by the project constraints, I am including the AI prompts used to generate the code for this repository.
The AI assistant (me) was prompted using the following interactions:

## 1. Initial Selection and Planning
**Prompt:**
> Dear Students,
> As you are aware, we have successfully completed the foundational learning sessions on Go (Golang). We are pleased to inform you that Infosys has now shared the project problem statements.
> Each student is required to choose one problem statement from the given list and complete the project individually using the Go programming language.
> ...
> [Attached PDF content with 10 Capstone Projects]
> 
> go through the pdf and choose the best problem statement that i have worked before and i have to do these in Go programming language. choose 1 best which is effective where judges will be improved by the features and robustness and easy to make

**Response:** I analyzed the 10 projects and chose **Capstone Project 5 (Event Registration)** because its primary challenge is concurrency control, which is incredibly well-suited to showcase Golang's capabilities. I generated an implementation plan and design document that highlights an Atomic DB Update Strategy to solve the race condition efficiently.

## 2. Execution Phase
**Prompt:** (Implicitly triggered by User approving the implementation plan with "Continue" or "LGTM")

**Response:** 
I generated the API utilizing:
1. Pure Go SQLite (`modernc.org/sqlite`) instead of CGO to guarantee cross-platform compilation without requiring the user or judges to install GCC/MinGW.
2. Standard Library HTTP Routing (`net/http`) utilizing the newly added Go 1.22 routing (`ServeMux`) enhancements `POST /path` to avoid heavy external frameworks.
3. An explicit integration test file `concurrent_test.go` that runs 100 concurrent mock-users using Goroutines to hit the critical race condition area, successfully proving that only exactly 5 registrations go through and 95 are gracefully blocked.

## 3. Review Feedback
**Prompt:**
> Continue

**Response:**
I wrapped up documentation, ensuring all components requested by the grading matrix (`README.md`, Design Document, Prompts file, DB schema initialization) are present in the final codebase.
