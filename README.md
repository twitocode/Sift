# Sift

A simple, but complete search engine

## Challenges

### Crawling

The system that I was using before used to work by choosing some seed URLs and then having permissions you can't upproof these URLs in the queue and then the queue would spawn spiders that would go to each of the seed URLs the seed URLs would then fetch all the content on that page, search for any other hyperlinks, and then add those hyperlinks to the queue. I would then have a counter for the number of passes.

did not want to get rate limited too fast, made a system where there is a pending pool of crawling jobs that have to wait for a channel labled by a hostname to then join that and be allowed to be executed

memory usage was exploding when crawling

switching to a domain filter system instead.