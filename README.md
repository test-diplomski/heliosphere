# heliosphere
Rate limiting for the system and it's applications

## Quickstart

1. Clone this repository

2. Run the following command

```
docker-compose up
```

## Endpoints

- GET http://localhost:8090/ratelimiter - retrieves all rate limiters
- GET http://localhost:8090/ratelimiter/{id} - retrieves a rate limiter by its id
- POST http://localhost:8090/ratelimiter - create a new rate limiter
- PUT http://localhost:8090/ratelimiter - update a rate limiter
- DELETE http://localhost:8090/ratelimiter/{id} - delete a rate limiter
- GET http://localhost:8090/ratelimiter/allow/{id} - checks if a rate limiter can allow request 