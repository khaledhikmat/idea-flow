## Run Locally

```bash
go mod init github.com/khaledhikmat/idea-flow
go get -u go.temporal.io/sdk
go get -u go.temporal.io/api

# terminal 1
temporal server start-dev
CLI 1.5.1 (Server 1.29.1, UI 2.42.1)

Server:  localhost:7233
UI:      http://localhost:8233
Metrics: http://localhost:57058/metrics

# terminal 2 - worker
go run worker/main.go

# terminal 3 - starter. Eventually this an API Layer
go run main.go happy
```

## Notes

You have several options:

1. You can use existing social media feature-rich collaborative platforms:
- Discord
- Slack
- Teams
- Ready System

2. Leverage existing multi-tenant systems:
- [https://www.judgify.me/l/pricing/](https://www.judgify.me/l/pricing/)

3. Design a platform that can be used for this purpose. Deploy and hope to get some revenue so u can fund your activities. But this requires time, money, team and commitment:

User Types:
- Idea submitters must signup/signin using a non-marketing site such as https://app.peace4handshake.com.
- Admins must also sign up.
- Judges are users but have been designated by admins to judge specific contests.
- Judges vote on every idea. The idea score is the sum of all votes.
- Sponsors or donors fund contests.

Entities:
- Ideas: Title, Body, Category, Genre, etc.
- Contests -> Ideas: Ideas always belong to a contest?

Workflows:
- Idea are onboarded: `Submitted` -> Categorized -> `Auto Approved`, `Auto Rejected`, `Human Approved` or `Human Rejected` -> Scored (wait for all judges) -> `Ranked` or `Sacked`. 
- Contest are onboarded: `Created` -> `Approved for Funding` -> `Being Funded` -> `Reached Goal` -> `Active`. Maintain leader board. 

User Experience:
- User -> Signup/Signin:
    - Search through available contests
    - Choose an active contest -> Submit an idea (limited to 5000 characters) in English only.
    - Search through existing categories
    - Submit an idea
- Admins -> Create Contest -> Setup goals, rules, awards and prices (FROM/TO).
- Admins -> Open Funding for Contests.
- Judges -> Explore Active Ideas and Score them within a perid of time.
- Donors/Sponsors -> Look for contests to fund and pledge. 
- Users -> User Interface (different than marketing i.e. app.peace4handshake.com) -> API -> Post Idea text and context ID -> Workflow

System:
- Authorizer
- API Endpoint to submit an idea against a contest: `POST /api/contests/{id}/ideas`.
- Workflow engine
- Database
- AI Subscription

Workflows:
- Contest
    - Created
        - Goal
        - Rules
        - Awards
    - Wait for Funding Approval
    - Approved
        - Start Receiving Pledges
    - Funded
        - Funding Goal Reached
    - Assign Judges - propably not
    - Active with Timeout
        - Receive Ideas
        - Receive Idea Scores
        - Formulate a Leader Board
    - Timeout: 
        - Produce a final Leader Board
        - Award them
    - Closed

- Idea
    - Submitted (includes parent contest ID).
    - Categorized
    - Auto Approved or Auto Rejected
    - Wait for Human Assessment
    - Human Approved or Human Rejected
    - If approved, notify Parent contest
    - Wait for Score or Timeout
        - On Score, Signal Parent to deliver score
        - The idea can accumulate comments but these are not part of the workflow execpt to count the number of comments
    - Timeout
    - Closed