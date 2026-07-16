# Sift

A simple, but complete search engine

## Challenges

### Crawling

The system that I was using before used to work by choosing some seed URLs and then having permissions you can't upproof these URLs in the queue and then the queue would spawn spiders that would go to each of the seed URLs the seed URLs would then fetch all the content on that page, search for any other hyperlinks, and then add those hyperlinks to the queue. I would then have a counter for the number of passes.