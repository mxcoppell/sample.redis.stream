# Contributing to Redis Stream Key Expiration Handler

First off, thank you for considering contributing to this project! It's people like you that make this tool better for everyone.

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to project maintainers.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the issue list as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* Use a clear and descriptive title
* Describe the exact steps which reproduce the problem
* Provide specific examples to demonstrate the steps
* Describe the behavior you observed after following the steps
* Explain which behavior you expected to see instead and why
* Include details about your configuration and environment

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, please include:

* A clear and descriptive title
* A detailed description of the proposed enhancement
* Examples of how the enhancement would be used
* Any potential drawbacks or performance implications

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code that should be tested, add tests
3. If you've changed APIs, update the documentation
4. Ensure the test suite passes
5. Make sure your code lints
6. Issue that pull request!

## Development Process

1. **Setting up development environment**
   ```bash
   git clone https://github.com/mxcoppell/sample.redis.stream.git
   cd sample.redis.stream
   go mod tidy
   ```

2. **Running tests**
   ```bash
   go test ./...
   ```

3. **Code style**
   - Follow standard Go formatting guidelines
   - Use `gofmt` to format your code
   - Run `golint` and `go vet` before submitting

## Pull Request Process

1. Update the README.md with details of changes if applicable
2. Update the DESIGN.md if you're changing core functionality
3. The PR will be merged once you have the sign-off of at least one maintainer

## Performance Considerations

When contributing, please keep in mind:

1. **Scalability**
   - Consider behavior with large numbers of keys
   - Test with various consumer counts
   - Monitor memory usage

2. **Race Conditions**
   - Be extra careful with concurrent operations
   - Use appropriate locking mechanisms
   - Consider edge cases in distributed scenarios

3. **Redis Best Practices**
   - Follow Redis performance best practices
   - Consider pipeline usage for batch operations
   - Be mindful of key expiration patterns

## Documentation

- Update documentation for any new features
- Include code examples where appropriate
- Document any new configuration options
- Update performance characteristics if changed

## Testing Guidelines

1. **Unit Tests**
   - Write tests for new functionality
   - Maintain existing test coverage
   - Include edge cases

2. **Integration Tests**
   - Test with actual Redis instance
   - Verify consumer group behavior
   - Test failure scenarios

3. **Performance Tests**
   - Benchmark critical operations
   - Test with various load patterns
   - Document performance results

## Questions?

Feel free to open an issue for any questions about contributing. We're here to help! 