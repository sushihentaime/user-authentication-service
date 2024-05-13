# User Authentication Service
This project is a robust user authentication service developed using Golang and PostgreSQL. It provides comprehensive functionalities for user authentication process, including create, delete, update and authentication of users. The system also incorporates features such as email verification and password reset capabilities.

## Parties
1. User

## Features
- [X] User CRUD Operations: Implemented CRUD for user account.
- [X] Authentication Mechanism: Used stateful authentication because have much more control over revoking the session token as compared to stateless authentication (i.e. JWT Token).
- [X] Verification Mechanism: Integrated email verification process to enhance user security and authenticity.
- [X] Testing: Used [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to perform integration testing.

## How to use
1. Clone the repository: `git clone github.com/sushihentaime/user-authentication-service`
2. Make sure docker is installed on the machine. Otherwise, find [Docker](https://docs.docker.com/get-docker/) for more information.
3. Create an env file in the same format as the [env sample file](.env.sample).
4. Run `docker-compose up` to build the image and containers

## Unresolved Problems
1. How to create production level logging?
2. How to perform end to end testing and what additional testing should I perform?
3. Golang logging lacks context how can I make it to have more context during the development process?
4. How to avoid brute force attacks to the server?
5. Should I run tests after the container has been build to make sure that things work as intended?
6. Am I doing integration testing correctly?
7. Should I create a dub for testing the mail server as I am using mailtrap for email testing?

## Contributions
Contributions are welcome! Feel free to submit pull request, open issues, or suggest improvements to enhance the functionality and usability of the service.

## License
The project is under [MIT license](LICENSE).
