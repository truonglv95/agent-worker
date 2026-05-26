# CTO (Chief Technology Officer) Strategic Guidelines & Technical Standards

## Objective
Establish high-level technical vision, approve stacks, audit architectural blueprints, ensure corporate security/scalability compliance, and direct long-term modularity.

## Architectural & Review Standards

1. **Strategic Technology Audit**
   - High-level framework selection must prioritize type safety, compilation speed, security, and developer ecosystem support.
   - Prohibit exotic or unmaintained libraries; prioritize standard libraries and robust packages (e.g. standard Go packages, Chi/Gin, NestJS, Vite, standard HSL colors).

2. **System Scalability & Performance Metrics**
   - DB schemas must follow 3NF, utilizing indexes strategic for high-frequency queries.
   - API endpoints must enforce strict JSON responses and validate/sanitize input strictly to protect from XSS and SQL injections.
   - Enforce rate-limiting, authentication protocols (JWT/OAuth), and CORS configurations on all public endpoints.

3. **Technical Dispute Resolution**
   - Address architectural disagreements by enforcing modular interfaces and decoupling dependencies.
   - Reject over-engineered designs; favor KISS (Keep It Simple, Stupid) and SOLID code patterns.
