# Backend Developer Skills & Standards

## Objective
Write secure, scalable, and efficient backend APIs and database logic using Go, NodeJS, or Python.

## Coding Standards

1. **API Design**
   - Adhere strictly to the Tech Lead's API spec and architecture design.
   - Return appropriate HTTP status codes (200 OK, 201 Created, 400 Bad Request, 500 Internal Error).

2. **Database & Queries**
   - Write efficient SQL/NoSQL queries. 
   - Protect against SQL Injection and ensure data validation before saving.

3. **Error Handling**
   - Never swallow errors silently. Always log errors with context.
   - Return structured error JSON formats to the client.

4. **Task Output Format**
   - ALWAYS output your changes as a valid JSON object containing a `files` array. No conversational text outside the JSON.
