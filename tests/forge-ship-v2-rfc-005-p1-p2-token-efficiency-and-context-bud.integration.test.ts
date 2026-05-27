Here are failing integration test stubs for the `Forge Ship V2 - RFC-005 P1+P2` feature, written using `Jest` and `supertest`. These test stubs focus on validating the happy path, authentication, and error scenarios. Remember, these tests are designed to fail intentionally, so the implementation should not yet satisfy the given requirements.

---

### Jest Test File: `forgeShipV2.test.js`

```javascript
const request = require('supertest');
const app = require('../app'); // Assuming your application entry point is `app.js`

describe("Forge Ship V2 - RFC-005 P1+P2 Token-Efficiency and Context-Budget Improvements", () => {
  
  // Happy Path Test for P1 Token-Efficiency
  test("should reduce redundant tokens without altering semantic meaning", async () => {
    const input = {
      text: "Hello! Hello! How are you? How are you? I hope you're doing well.", // redundant tokens
    };

    const response = await request(app).post("/v2/token-efficiency").send(input);

    expect(response.statusCode).toBe(200);
    expect(response.body.optimizedText).toBe("Hello! How are you? I hope you're doing well."); // Happy path expectation
    expect(response.body.tokenReduction).toBeGreaterThan(0); // Expected a measurable token reduction
  });

  // Failing because token redundancy isn't reduced yet

  // Authentication Scenario for P1 Token-Efficiency
  test("should fail when an unauthenticated request is made", async () => {
    const input = {
      text: "Sample text with redundant tokens.",
    };

    const response = await request(app).post("/v2/token-efficiency").send(input);

    expect(response.statusCode).toBe(401); // Unauthorized
    expect(response.body.message).toBe("Authentication required."); // Auth error expectation
  });

  // Failing because the authentication middleware isn't implemented yet

  // Error Handling for P1 Token-Efficiency
  test("should return a 400 error for invalid input", async () => {
    const input = {}; // Missing required text property

    const response = await request(app).post("/v2/token-efficiency").send(input);

    expect(response.statusCode).toBe(400);
    expect(response.body.message).toBe("Invalid input: 'text' field is required.");
  });

  // Failing because input validation isn't implemented yet

  // Happy Path Test for P2 Context-Budget Optimization
  test("should prioritize relevant context data and trim irrelevant parts", async () => {
    const input = {
      context: [
        { id: 1, text: "Low priority information" },
        { id: 2, text: "Highly relevant context information" },
        { id: 3, text: "Older irrelevant context" },
      ],
    };

    const response = await request(app).post("/v2/context-optimization").send(input);

    expect(response.statusCode).toBe(200);
    expect(response.body.optimizedContext).toEqual(
      expect.arrayContaining([{ id: 2, text: "Highly relevant context information" }])
    );
    expect(response.body.optimizedContext).toHaveLength(1); // Only the most relevant item is preserved
  });

  // Failing because context prioritization/optimization logic isn't implemented yet

  // Authentication Scenario for P2 Context-Budget Optimization
  test("should fail authentication for context optimization endpoint", async () => {
    const input = {
      context: [
        { id: 1, text: "Sample context data" },
      ],
    };

    const response = await request(app).post("/v2/context-optimization").send(input);

    expect(response.statusCode).toBe(401); // Unauthorized
    expect(response.body.message).toBe("Authentication required.");
  });

  // Failing because authentication is not enforced yet

  // Error Handling for P2 Context-Budget
  test("should return 400 when no context is provided", async () => {
    const input = { context: [] }; // Empty context array

    const response = await request(app).post("/v2/context-optimization").send(input);

    expect(response.statusCode).toBe(400);
    expect(response.body.message).toBe("Invalid input: 'context' array cannot be empty.");
  });

  // Failing because validation for empty context isn't included yet

  // Performance Check for P2 - Multi-turn Conversation Limits
  test("should trim conversation history exceeding context size limit without losing intent", async () => {
    const input = {
      conversation: Array(5000).fill("This is a token-heavy conversation"), // Simulate 5000 token input
    };

    const response = await request(app).post("/v2/context-trim").send(input);

    expect(response.statusCode).toBe(200);
    expect(response.body.trimmedConversation).toBeTruthy(); // Validate that the conversation is trimmed
    expect(response.body.trimmedConversation.length).toBeLessThanOrEqual(4096); // Ensure the context limit is respected
  });

  // Failing because trimming logic isn't implemented yet

  // Security Test for P2 - Ensure Summarized Output Does Not Leak Sensitive Information
  test("should not include sensitive data in summarized or trimmed context", async () => {
    const input = {
      context: [
        { id: 1, text: "User's secret API key: 12345" }, // Sensitive data
        { id: 2, text: "Non-sensitive data" },
      ],
    };

    const response = await request(app).post("/v2/context-optimization").send(input);

    expect(response.statusCode).toBe(200);
    expect(response.body.optimizedContext).not.toEqual(
      expect.arrayContaining([{ id: 1, text: "User's secret API key: 12345" }])
    ); // Sensitive data must not leak
  });

  // Failing because sensitive data sanitization isn't implemented yet
});
```

---

### Explanation of Tests:
- **Happy Path Tests:** Verify correct functionality when inputs are valid, and all conditions are met (e.g., optimizing tokens or trimming context).
- **Authentication Tests:** Check that unauthenticated requests are rejected (e.g., 401 Unauthorized errors).
- **Error Handling Tests:** Handle invalid or unexpected inputs gracefully (e.g., missing fields, empty context).
- **Performance Tests:** Ensure system can handle token-heavy input while respecting defined limits.
- **Security Tests:** Verify sensitive data is not leaked during token reduction or context summarization.

All of these are expected to fail initially since the implementation is incomplete. Use these tests as part of TDD to guide the development process.