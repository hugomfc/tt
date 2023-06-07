docker build -t tt-redis -f Dockerfile.redis .
docker run --rm --name tt-redis-instance -d -p 6379:6379 tt-redis
