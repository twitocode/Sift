# Sift

A simple, but complete search engine

Goal: make a project on the larger side without AI writing any code

## Challenges

### Crawling

The system that I was using before used to work by choosing some seed URLs and then having permissions you can't upproof these URLs in the queue and then the queue would spawn spiders that would go to each of the seed URLs the seed URLs would then fetch all the content on that page, search for any other hyperlinks, and then add those hyperlinks to the queue. I would then have a counter for the number of passes.

did not want to get rate limited too fast, made a system where there is a pending pool of crawling jobs that have to wait for a channel labled by a hostname to then join that and be allowed to be executed

memory usage was exploding when crawling

switching to a domain filter system instead.

frontier system god file

concurrency errors

adding a bloom filter to check for deduplication

- using bitwise operations for the first time

So the original system with Bloom filters used to use an array of bytes which with a size of how many elements I expect to be crawled and where each byte represented the each indices in the Bloom filter. The problem with the system is that it uses way too many bytes. So, what I did instead was I used what's called a bit set, and this bit set has an array of bytes, but we don't treat each indicate in the array as a number. What we do instead is we treat each bit inside each byte as a number. So bits 0 to 7 would represent one byte number zero, and then bits eight to fifteen would represent byte number one, and then this can scale up to whatever number I need.

when going through performance issues i noticed that i kept making goroutines when it was not needed. I also did not have proper cancelation procedures when shutting down; leading to many channel sending errors.

As of July 22, 2026 the program can crawl 10,000 pages with 150 spiders crawling.
