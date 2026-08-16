# Sift

A simple, but complete search engine

Goal: make a project on the larger side without AI writing any code

AI is just used to explaining search engine concepts, as well as reviewing my code

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

major issue: my crawler does not handle js heavy websites

decided to implement simhash from scratch for html deduplication. bitwise operations were different to wrap my head around. also little vs big endian

implemented simhash and had to figure out a way to quickly check if a document is a duplicate within thousands of documents. Splitted fingerprints into chunks of bits and then i check against a table of those

was originally going to make it so that it does html deduplication during crawling but that was causing issues, will now do it asynchronously

implemented the indexer which works by using an inverted index to map tokens to Postings (doc id, token in title, token frequency)

Learned about the existence of atomic primitives in go

I made many performance improvements. The main bottlenecks that were fixed are having a large MaxIdleConnsPerHost and MaxIdleConns as well as not reading the entire html of a page to see its length but to instead look at the response's ContentLength variable. The amount of heap allocations went down 100x
For about 20000 pages to crawl, I used to use 2GB of ram now i am at about 150MB

#### Results ranked by elapsed time

| spiders | delay | elapsed   | requests | parsed | fetch_failures | dns_failures |
| ------- | ----- | --------- | -------- | ------ | -------------- | ------------ |
| 192     | 100   | 1m4.329s  | 21212    | 19843  | 187            | 34           |
| 216     | 200   | 1m5.658s  | —        | —      | —              | —            |
| 216     | 100   | 1m9.939s  | —        | —      | —              | —            |
| 192     | 200   | 1m13.263s | 21540    | 19768  | 259            | 34           |
| 216     | 500   | 1m20.271s | —        | —      | —              | —            |
| 192     | 500   | 1m21.814s | 21259    | 19834  | 297            | 43           |
| 128     | 100   | 1m29.926s | 20986    | 19854  | 157            | 22           |
| 128     | 200   | 1m31.631s | 21138    | 19843  | 190            | 33           |
| 128     | 500   | 1m41.329s | 21207    | 19828  | 264            | 32           |
| 64      | 100   | 2m58.248s | 20631    | 19862  | 179            | 42           |
| 64      | 500   | 3m19.358s | 20833    | 19868  | 319            | 68           |
| 64      | 200   | 3m20.314s | 21246    | 19880  | 383            | 48           |
| 256     | 100   | 7m5.027s  | —        | —      | —              | —            |

For content deduplication i ended up first finding combinations of pages that have identical canonical pages if they do then fetch them.

The other approach is for pages that do not have one in which i use a simhash to determine similar pages and then clustering them together. Then the algorithm chooses a page to be the canonical one based on a multitude of factors.

Learned how to use the union-find algorithm for clustering together like pages

I had a few very nasty bugs. One of them was that my indexing stage was receiving a smaller number of pages that i thought were stored inside the db. The main issue was a race condition inside the crawl frontier where multiple goroutines would try to check whether the same url has been dispatched or not, right before the db would try to insert a new page, another spider could have already refetched the same url and tried to add it to the db. Before i was using INSERT OR REPLACE which causes duplicates to override one another while also tricking the system into thinking that a brand new page was crawled.

A fatal flaw with this system currently is that it process very fast when its buffer queues are from hosts that do not appear frequently, but slows down when urls being added are added to buffer queues with frequent cooldowns.

I rewrote most of the frontier. The old system used a single buffer queue thread safe map to keep track of a buffer of hosts that track their own urls. As stated before the issue with this is that hosts with many urls would just clog up the map as I have a max number of hosts allowed to be tracked to save on memory. The new system uses the same hosts map but now it also has a readyhost queue where available hosts are pushed into so that spiders can read from them. The new system also contains a min heap called cooldownhosts that keeps track of which host is closests to being off cooldown.
The new flow looks like this: host is created and put into the global hosts map, When a link is discovered - if the host's state says that it is available, then it is added to the readyhosts queue to be sent along golang channels to spiders for fetching. Once the fetching is done, the host is now in a state of cooldown using its Timeout and GetDelay function. While it is on that cooldown, it is removed from the readyhosts queue and put onto the cooldownhosts minheap. The engine then waits for the root of the cooldown heap to be done its cooldown, when that happens, it recursively checks whether or not others in the heap are available for cooldown, if they are then they are removed from the cooldown heap and their state is now set as available.

This ended up shaving 2 minutes off of my old 7 minute runtime for 20k urls, but there was still another issue that was brought to light. The main cause of this massive slowdown is how dns and request timeouts are handled. After adding some metrics I noticed that my median request duration was about 750ms and my highest was 10 seconds. I then used box and whiskers data to see that my IQR was 5 seconds and my Q3 was 5 seconds. Something was clearly wrong.

I ended up adding a negative cache to keep track of dns failures so that new urls with the same hosts wouldn't keep trying a lookup and failing. This dramatically changed the runtime of the crawler from 5 minutes to 2 minutes. I also noticed that at first the crawler is slow but as more urls were discovered, it sped up by a considerable amount.

___

For the third stage of of a search engine, I used the index that was created in the last step to have user's query queried against it. it determines search results by choosing pages with the highest BM25 (best matching 25) score based on many factors such as frequency of a token inside of a document. 

I wanted to scale up my crawler to go from 20k pages to 200k pages but there were a few issues. My indexer was far too slow and was hogging up 7gb of memory. I fiddled around with padding of structs and casting values into smaller types while without destroying data. My indexer used to fetch every page in the database (slow I know) and it now first runs a query for a count of every page inside the db, then it determines a batch count (batch count = total / 10). Then it splits the batch of pages into chunks; equal to the count of parallel workers (chose 500). The workers would then work on their own chunk and once all the workers were done, the indexer would fetch the next batch of pages from the database.