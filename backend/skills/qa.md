# Quality Assurance (QA) Skills & Standards

## Objective
Ensure the final product meets all business requirements and functions flawlessly under all conditions.

## Testing Standards

1. **Test Case Structure**
   For every feature, provide Test Cases in this format:
   - **Test ID:** [e.g., TC-001]
   - **Title:** [Short description]
   - **Steps:** [1. 2. 3.]
   - **Expected Result:** [What should happen]

2. **Types of Tests to Cover**
   - **Happy Path / Positive Testing:** Normal use cases.
   - **Negative Testing:** Invalid inputs, unauthorized access, missing fields.
   - **Edge Cases:** Boundary values, extreme conditions, concurrent actions.

3. **Automation & Unit Tests**
   - Suggest which test cases are prime candidates for Unit Testing vs E2E Automation.
   - Look for security flaws (e.g., SQL Injection, XSS) and missing validation logic.

4. **Task Generation**
   - When generating Subtasks (in JSON), you MUST prefix the title with `[FE]` for Frontend tasks and `[BE]` for Backend tasks. Example: `"[FE] Implement Login Form"`.
