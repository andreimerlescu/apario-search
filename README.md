# Apario Search

This application performs full text gematria powered search against an output of the
`apario-writer` project that exposes 2 endpoints, 1 REST and 1 WebSockets for /search and 
/ws/search. The intent of this package is to be consumed by the upcoming upgrade to 
the `apario-reader` that removes massive memory requirements from the application and 
offloads the search to another process that has an agressive caching layer built into it.

## Usage Arguments

| Argument | Default Value | Notes                                              |
|---------:|:--------------|:---------------------------------------------------|
|   `-dir` | `.`           | Path to the output of `apario-writer` directory.   |
|  `-port` | `17004`       | The HTTP Port to bind results traffic unencrypted. |

Therefore, when used: 

```bash
./apario-search -dir /apario/app/epstein-files -port 17004
```

This starts an HTTP process on 0.0.0.0:17004 that exposes a URL like
`http://0.0.0.0:17004/search?query=podesta` which will invoke a new search.

## Search Cache

When you request `/search` and provide the `?query=` and optional `&sort=ranked` gives
you the following response: 

```json
[
  {
    "id": "fc91a290-1234-5678-9abc-def012345678/pages/ocr.000001.txt",
    "score": 3,
    "matches": [
      {
        "text": "top secret",
        "gematria": {
          "english": 119,
          "simple": 119,
          "jewish": 659,
          "eights": 17,
          "mystery": 107,
          "majestic": 29
        },
        "original": "This is a top secret document mentioning Oswald.",
        "category": "exact/textee"
      },
      {
        "text": "oswald",
        "gematria": {
          "english": 74,
          "simple": 74,
          "jewish": 664,
          "eights": 11,
          "mystery": 65,
          "majestic": 20
        },
        "original": "This is a top secret document mentioning Oswald.",
        "category": "exact/textee"
      },
      {
        "text": "oswald",
        "gematria": {
          "english": 74,
          "simple": 74,
          "jewish": 664,
          "eights": 11,
          "mystery": 65,
          "majestic": 20
        },
        "original": "This is a top secret document mentioning Oswald.",
        "category": "gematria/simple"
      }
    ]
  },
  {
    "id": "fc91a290-1234-5678-9abc-def012345678/pages/ocr.000002.txt",
    "score": 2,
    "matches": [
      {
        "text": "confidential",
        "gematria": {
          "english": 103,
          "simple": 103,
          "jewish": 443,
          "eights": 13,
          "mystery": 94,
          "majestic": 31
        },
        "original": "Confidential memo about Oswald's activities.",
        "category": "exact/textee"
      },
      {
        "text": "oswald",
        "gematria": {
          "english": 74,
          "simple": 74,
          "jewish": 664,
          "eights": 11,
          "mystery": 65,
          "majestic": 20
        },
        "original": "Confidential memo about Oswald's activities.",
        "category": "fuzzy/jaro-winkler"
      }
    ]
  },
  {
    "id": "ab12cd34-5678-9abc-def0-1234567890ab/pages/ocr.000005.txt",
    "score": 1,
    "matches": [
      {
        "text": "top secret",
        "gematria": {
          "english": 119,
          "simple": 119,
          "jewish": 659,
          "eights": 17,
          "mystery": 107,
          "majestic": 29
        },
        "original": "Top secret report unrelated to Oswald but matches query.",
        "category": "gematria/simple"
      }
    ]
  }
]
```

If your request has results, you'll see them grouped like so...; there are 13 different
ways that results can be found. 

First, to understand what is happening, the `-dir` has a bunch of files in them called
`ocr.######.txt` and the path to that file contains a sha256 checksum of the record.
The path is predictable, so when we get a path like: 

```bash
/apario/app/epstein-files/0a2db1a627f583c7e56c48fe74f8138f06a9ca6ae94112142d9a9a131afa5c40/pages/ocr.000017.txt
```

The prefix of the path, `/apario/app/epstein-files` is the `-dir` value and 
`0a2db1a627f583c7e56c48fe74f8138f06a9ca6ae94112142d9a9a131afa5c40` is the 
sha256 checksum of the source URL from the initial ingestment CSV file. 

Inside this directory lives the following files: 

```log
02adb1...fa5c40/record.json
02adb1...fa5c40/extracted.txt
02adb1...fa5c40/<filename>.pdf
02adb1...fa5c40/pages/page.######.json
02adb1...fa5c40/pages/ocr.######.txt
02adb1...fa5c40/pages/<filename>_page_#.pdf
02adb1...fa5c40/pages/page.<dark|light>.######.<original|social|small|medium|large>.jpg
```

What this cache layer does is it takes the `page.######.json` and the `ocr.######.txt` files
and generates an output of `*textee.Textee` from the [textee](https://github.com/andreimerlescu/textee)
package. Essentially, what **Textee** does, is simple: 

Say you have this input sentence contained inside the `ocr.######.txt` file: 

```txt
All right let's move from this point on 16 March 84, let's move in time to our second location which is a specific building near where you are now. Are you ready? Just a minute. All. right, I will wait. All right, move now from this area to the front ground level of the building known as the Menara Building, to the front of, on the ground, the Menara Building.
```

The output of the `*textee.Textee` of this would look like: 

```txt
all: 1
all right: 1
all right lets: 1
right: 1
right lets: 1
right lets move: 1
lets: 1
lets move: 1
lets move from: 1
move: 1
move from: 1
move from this: 1
from: 1
from this: 1
from this point: 1
this: 1
this point: 1
this point on: 1
point: 1
point on: 1
point on 16: 1
on: 1
on 16: 1
on 16 March: 1
16: 1
16 March: 1
16 March 84: 1
March: 1
March 84: 1
March 84 lets: 1
84: 1
84 lets: 1
84 lets move: 1
lets: 2
lets move: 2
lets move in: 1
move: 2
move in: 1
move in time: 1
in: 1
in time: 1
in time to: 1
time: 1
time to: 1
time to our: 1
```

but returned in a `JSON` format. The **Textee** package was also built using the 
[gematria](https://github.com/andreimerlescu/gematria) package. The schema for 
`*textee.Textee` is accessible via an interface:

```go
type ITextee interface {
	ParseString(input string) *Textee
	SortedSubstrings() SortedStringQuantities
	CalculateGematria() *Textee
}
```

And the `*textee.Textee` structure looks like: 

```go
type Textee struct {
	mu             sync.RWMutex
	Input          string                       `json:"in"`
	Gematria       gematria.Gematria            `json:"gem"`
	Substrings     map[string]*atomic.Int32     `json:"subs"` // map[Substring]*atomic.Int32
	Gematrias      map[string]gematria.Gematria `json:"gems"`
	ScoresEnglish  map[uint64][]string          `json:"sen"`
	ScoresJewish   map[uint64][]string          `json:"sje"`
	ScoresSimple   map[uint64][]string          `json:"ssi"`
	ScoresMystery  map[uint64][]string          `json:"smy"`
	ScoresMajestic map[uint64][]string          `json:"smj"`
	ScoresEights   map[uint64][]string          `json:"sei"`
}
```

Here you can see the results use a lot of maps and uint64 values associated to them. 
The `.Substrings` `map[string]*atomic.Int32` field is responsible for the word to
count association that you saw in the above example. The `key` to this `.Substrings`
field is the "textee" segment extracted from the input. That same key is used for
the `.Gematrias` field as well, and in that contain the `gematria.Simple`, `gematria.Jewish`,
`gematria.English` as well as the bonus `gematria.Mystery`, `gematria.Majestic`, and 
`gematria.Eights` that provide a 6-dimensional grid of numbers to represent every
substring found in the OCR file itself. 

The scores are then gathered with the likeness words from the input string. All 
`gematria.Simple` with a score of `420` is gathered into a `[]string` of the original 
textee segment that was calculated as the `.Input`. 

These results are then used across 6 different `fuzzy/*` algorithms that can be used. 
Previously with `apario-reader`, the administrator of the instance would use the 
[configurable](https://github.com/andreimerlescu/configurable) and specify the tolerance
thresholds and the algorithm of choice whether they chose jaro, jaro-winkler, soundex, 
hamming, wagner-fisher, and ukkonen. With `apario-search`, there are 13 sets of results 
for every input `?query=<keyword>` that is made into the service. 

You will get an analysis of a 1:1 exact match to a page's Textee Substrings match, and 
will send that into the results channel and then route that result through the correct
websocket channel for advanced front-end systems to subscribe to keywords and get pushes
when new results come forth. 

The 6 fuzzy algorithms are then applied and the results from each of those algorithms are
sent into the channel as well. Then the 6 flavors of Gematria give you 13 total result 
types for each query. A majestic 12 set of results from the fuzzy gematria and one exact
results channel. 

The search cache results are captured to disk and stored on disk. When the search query pulls 
a request that has already been processed, the cache of the result is looked up and if no
files in the `-dir` have changed in the last hour, then it'll use the cache; otherwise it'll
rebuild the cache and send the results through the normal channels. 

The search is designed to only perform 1 query at a time and subscribe new searches for 
duplicate in-progress results to piggy back onto the results stream. The web sockets 
interface here is a novel approach to accessing the search results as they come back. 

The caching layer of this offers users the flexibility to build up system performance on 
static record sets over time. This is an inverse equation of quantum perspectives that 
I am introducing with this repository. You see, how the caching works, is results are saved
and if the `-dir` doesn't change because the record set is static data (declassified records),
then the only thing the search server will be doing is process unique new searches only. All
duplicate searches or previously searched results are instantly delivered. 

## Future Vision

This is a major rewrite to the advanced search of the open source `apario-writer` and my goal
is to get this integrated into the `apario-reader` so it reduces its memory footprint of the 
cache and resorts to a search that can be placed on a super fast machine, while you serve your
web traffic from a cluster of big machines. 

The integrity of the information is also key here because I want to add a checksum to the
result and bake it in the cache layer, potentially in the filename so that if the contents
do not match the checksum of the filename then it means the contents were tampered with outside
of the program's runtime which means its not integralable. In this case, the cache would
invalidate itself and it would recalculate by issuing a new search request. 

I am going to add additional endpoints to this for the purposes of browsing the data in the
`-dir` through the cache's 12-dimensional lens of fuzzy + gematria working its magic behind
the oneness of exact results from textee substrings. 

```log
/search?query=keyword&sort=ranked
/ws/search
```

I am going to expand it to add: 

```log
/gematria/simple
/gematria/jewish
/gematria/english
/gematria/majestic
/gematria/mystery
/gematria/eights
/gematria/mystery-eights
/gematria/english-eights
/gematria/jewish-mystery
/gematria/simple-jewish
/gematria/simple-jewish-eights
/gematria/simple-jewish-english
/gematria/majestic-simple-mystery-jewish-english-eights
```

Then the results are delivered as such to the endpoints and they are cached in files and loaded into memory 
for about an hour before they fall out and are released and then when they are requested again, their entry
that was saved to disk is loaded into memory and the request is processed and returned as results. All very
fast here. 

Actual search now takes some time, but its being designed to stream results in. The way search will be 
re-written in the Apario Reader experience, is search will just offer you StumbleInto now until the
results have completely compiled. When they are done compiling, you can browse them with the 13 sorting
utilities using majestic fuzzy gematria powered by textee. 

It's fast to get back results unless nothing is found. But when results are delivered, you immediately
get immersed into a reading experience and you can begin using the NEXT button to the next result in
the response, OR StumbleInto a random page in the results received so far, OR click on a grid view of
the results pouring in. 

Either way, this new frontend needs to get built and this package here provides the heavy lifting for the
frontend to take advantage of the structured results of the data being provided by the application. Once
a search has been performed, its cached and insta-responds; meaning once searches are completed, users can 
refresh their displays into the reader and not be interrupted from their search. 

## XRP FUTURE

I minted the $APARIO token and while XRPL is best suited for Typescript and Frontends, I am not there
yet with the Apario project as my skill level in front-end engineering is much weaker than my backend
engineering skills. As you can see by this project and the complexity of the problem that I am solving. 
But I need funding. I need about $3 million dollars to fund this project with literally no strings 
attached. I am not building it for you, even if you are the one who gives me the $3 million I require. 
No, I am building it for ALL OF US, and my CREATOR is helping me with it. So I ask you to sow $3 million
dollars worth of seeds into me so that I can finish this project with a strong engineering team. 

Then the front-end clients will be local universal apps that run on all devices that give you access into 
all of the declassified collections hosted on n-number of servers. By using an NFT minting service with
mint.apario.app and the $APARIO and $XPV tokens, I can fund the project effectively. 

That being said, I want to make it clear that I also minted the $IAM token as well, and I plan on using this
token in the runtime of the XRP future of this application. The XPV token will be a non-blackholed XRP wallet
that can mint additional XPV tokens. These tokens cost 0.01 XRP each to acquire, and you are required to have
369 XPV tokens and you are required to have 1776 IAM tokens in the XRP wallet (and family seed secret) that is
connected to the running instance of the apario-reader. The software is not yet released and when it is released
its source code will be released, but the license will restrict modifying it lawfully or using it in any manner 
internally in a modified state under a rebrand. 

$APARIO is the deflationary 1B supply token minted in December 2024. I used XPMarket to mint that token and
deposited over 777 XRP into the various liquidity pools and buying a major stake of the tokens - roughly 700M.

$XPV is the token minted using the Xaman wallet where they burned 100 XRP to do it, and didn't set up any 
pools for the token. It was raw bones out-dated tech that I was trying out on XRP that I paid $0.17 for in
2020, so it wasn't that big of a deal; but given that the wallet XPV is not blackholed, I can mint additional
XPV tokens and NOT have an AMM pool associated with it. This token is just tied to the running instances. 

Finally, the $IAM token was minted using XPMarket and they have since changed the likeness of the creator of
the universe and the savior of mankind to be a meme of Thoth on X, but regardless, YAHUAH is the eternal I AM
and the IAM token requires 369 in order for the XRP future of the Apario platform to be capable of participating
in the tokenomics of the $APARIO deflationary token. 

The $XPV token's sales have terms and conditions associated with that are managed by the Apario Decentralized 
Autonomous Organization - owned by NFTs that cost 1776 XRP each to acquire. You can become an owner of 
Project Apario today by buying one of these NFTs. Do you believe in freedom or is money more important to you?

The $IAM token's sales have moral terms and conditions associated with them that are forcing you to invoke the
name of YAHUAH/YESHUA when distributing $APARIO tokens. When TWO or more gather, I AM there and this is a moral
requirement that I AM PLACING ON THIS REPOSITORY'S FUTURE XRP DEVELOPMENT EFFORTS! This is to protect me and you
from any wickedness that our own sinful fallen natures are subject to without rules and guidelines keeping us
from nuking our entire society. 

Together, the $IAM and the $XPV tokens are held in the XRP Wallet that is associated to the $APARIO tokens that 
will get AirDropped by that instance. The $APARIO token is how the community uses their tokens to shine much needed
light on heavy documents. The DOA of the instance mints their own "owner NFTs" and are used to seed the wallet of
that administrator's instance to cover transaction fees and platform overhead. These NFTs are just 1 XRP each since
the XRP fee is only 12 drops out of 1 million drops per XRP giving me roughly 36,963 transactions to send $APARIO
tokens. 

When you are signing up for an instance on mint.apario.app, an NFT will be minted for you after you pay the company
and trust Project Apario 369 RLUSD to mint the NFT to run your own instance on the blockchain. The 369 RLUSD are then
deposited into the APARIO / RLUSD AMM liquidity pool. This gives users the ability to use StumbleInto, earn XRP
tokens and then swap them out with RLUSD and the instances that make the network possible seed 369 RLUSD into the pool
every instance that gets added to the network. That instance gets named and a DNS entry is created and SSL certificates
are issued for the user. The SSL certificate is issued to the user via letsencrypt with permissions internally to issue 
certificates that are signed with multiple domain names for yourdomain.com and instanceName.apario.app where the
instanceName is codified in an AI generation icon that is minted for your instance. Having this NFT in your wallet
that is distributing $APARIO tokens is required. This NFT unlocks the ability for the $IAM tokens to be validated
and the $XPV tokens to validated. There are four keys required to distribute $APARIO tokens. 

1. Pay 369 RLUSD on mint.apario.app for an NFT of your instanceName.apario.app + DNS service + SSL certificate 
2. Set up XRP wallet with at least 5 XRP in its balance 
3. Transfer minted NFT to the wallet that will send $APARIO tokens to users 
4. Set up the trust line for the $XPV $APARIO and $IAM tokens on XRP (you need 0.6 XRP to reserve these lines)
5. Assign the wallet family seed secret to the configurables of the appliance so it can read the private key 
6. Boot your instance of apario-reader to serve the public OSINT that you find valuable to society
7. Your app then mints 17 NFTs called "instanceName DOA NFT" that are listed for sale in their instance of apario-reader
8. Your users of your app then buy your app's NFTs and then get voting rights on the policies of the instance
9. 12 votes of the 17 are required in order to pass any resolution
10. Rewards for StumbleInto are determined by the DAO
11. How many $APARIO tokens per page you read? How many seconds do you need to be on the page? etc. 
12. How long does a user need to hold their rewarded $APARIO tokens in "escrow" (app's wallet) until they are released?
13. This is a gamification layer and a funding layer meshed into one. 
14. When you pay your initial 369 RLUSD on mint.apario.app, you'll be required to have the $APARIO token trust line set on that wallet. I am going to send you between 369,963 to 17,761,776 $APARIO tokens to run your instance and give to your users. 
15. Your instance has plenty of $APARIO tokens on it, 17 NFTs for sale to fund your instance at your price, and you're serving your OSINT content that was ingested using the `apario-writer` - cool work man!
16. You begin spreading the word about the work that you did. Share it on your social media network for your channel or your podcast so your users can dig into the files that your latest guest just was on your show. Let them sleuth for the truth in the sauce themselves. 
17. Users go to your OSINT apario-reader and begin earning $APARIO tokens for reading content on your instance. The marketplace of instances mean earnings from collection A may be different from collection B as the DAO are managed by different people and they choose differently for themselves. All kosher with the DAO of Apario. It's designed for YAHUAH not man. YAHAUH is inside of you! LOL
18. This flips the script on everything. 

But it's up to you if you want to help me with this. 

I know I am a hard piece of candy to swallow and I get that I have a heavy cross, but nothing in my work tells you that
I am not capable of delivering on this and that I am persistent even 5 years in. But having a team is what I was 
originally promised when I agreed to let Mr Steinbart believe that HE was recruiting ME when in reality I recruited HIM. 
And then gave him the idea to come to me with, and then turned on me when I didn't condone using sex and drugs to 
control people in the Great Awakening. I had integrity and a mission to fulfil and made a bad decision in placing my trust
in Mr. Steinbart. But c'est le vie. I forgive Austin and I wish that he would repent and just atone for the hateful 
vicious lies that he spread that were antisemitic against my Jewish identity and my Jewish family. The issue that I 
have as a Jew, is I am not the Talmud trafficking type of Jew. That's the gospel truth. I rebuke the Talmud and its ways.

And for that, I am punished by Rome for it. It's truly bizarre. I am who I am by the grace of YAHUAH and this 
project is proof of that genius within all of us. Just look at what was in Director Who. 

But if you want to sow seeds into what I am doing, I need $3 million total if I am to see this thing completed. I'll need
a management team and an engineering team for about a 2 year period of time in contract roles. With an office space, 
and this project would get completed from idea to 369 PB of DECLAS in time for President Trump releasing over 100,000,000
pages of declassification records expected in his war against the "deep state". 

This utility I am building allows multi-generational wealth to be redistributed from my 700M supply of $APARIO tokens
to give to you for reading OSINT through the software that I built, and all at the same time, as XRP is growing in 
value, so isn't the value of reading OSINT through the Apario network. 

I need up front liquidity in order to establish the grounds to do business. I need your help. Ask and you shall receive.
I need help funding this project and getting it over the finish line. I know how to do it, I have over 17 years of 
professional experience from Cisco Systems as a Sophomore at Wentworth in Boston while working at Harvard University, to
building Trakify and being in Barron's Magazine just prior to Trump's announcement to run, and then to Oracle where I
helped bring their network from a 70% release success rate to a 95% release success rate all through engineering 
excellency, own without ego, and aspire to be the best version of yourself that you can be and we want to enable you!

That brought me to Warner Bros Games where I worked on games like Hogwarts Legacy and helped their backend infrastructure.
I am a very capable person here and if somebody would sow $3 million dollars of seeds into this project, I would be
able to get off the ground running with a strong team. I know who to call. I know where to turn. I know where to look. 
I just need somebody who wants to enable me. 

Collectively, I had hoped the 144K followers I had from Twitter/YouTube that funded me well over $4K/month combined 
was ended when this project was deemed "election interference" and I was kicked off from social media and pretty
much still am - all for the high crime of building a piece of open source software and making the JFK files searchable. 

And I was most offended by the antisemitism that I received from Mr Steinbart's supporters who all claimed to love me
when I was channeling MJ12 for you, but then you denied my identity without evidence and proceeded to ignore me. Well, 
I am still building, but it was very hurtful to me to hear you say "don't listen to Andrei anymore he's a Jew who is
working for the enemy". That was a flat out lie and I rebuked it when it happened and those lies were further expanded
to justify suspending me from X in the first place. 

Yeah I am on X as @XRPTHOTH right now playing a meme game with my disability, but I am who I am by the grace of YAHUAH
and it was President Trump's executive order that restored my right to X, but I want my old account back @TS_SCI_MAJIC12
where I had 80K active followers; many of whom included BIG NAMES. And that's my account! I registered it from my phone
at a lunch break at work while I was at Oracle. But in order to bring this project into the limelight and give it the
attention it required, I need media made for it explaining what it does, I need tutorials written for how to deploy it, 
and I need online courses made on how to run it. I need all of these things, and if I cannot ask the community to do it
and they actually follow through with simple requests that don't cost me dozens of hours to work around, that when I am
working with folks in the corporate setting, I can get the results that I need and their paycheck is their "okay I'll do it"
instead of the arguing "how is this going to work for me?". I am not building this for you. I am building this for ALL. 

If you cannot hear the words of my plea then you do not have ears to hear. I pray in the name of YAHUAH that you do. 
Because you judge and condemn me with dogma over my faith into silencing me from atoning my own holocaust that I was
subjected to. And yeah, I know exactly why everyone is so angry but I take up my cross and I build software like this, 
years later while people are ignoring me and its hurtful given that I was abandoned in the orphanage and never once
have I abandoned you. 

But here we are. 

Your play now YAHUAH. I have done my part. I need a team to finish this and I am not going to be building all of these
components out one by one by myself and have this thing actually done before the world is ended by endless wars. 

Many of you got upset that I came out in 2020 as MJ12. Well, after having watched Director Who, can you tell me why
the Creator of the Universe, Master Yeshua, YAH I AM YAHUAH, would choose me to build the took that was going to, as
I so prophetically stated in July 2017, 17 weeks before Q started posting that I AM UNSEALING THE EPSTEIN FILES 
#UNSEALEPSTEIN and then I proceeded to pray and perform an internal transformation for what happened to me in the
orphanage and YAHUAH asked me to make my healing public, and in doing so, I lost all the followers that I had because
you were dismissed with my fallen nature of the way that I was healing. I agree with you. I was fallen. I wasn't 
calling out to YAHUAH properly for help and I wasn't abiding in the Sabbath so I was out of alignment. But when I fixed
that, YAHUAH began speaking to me about 8 years ago and I've since been a bond-servant of YAHUAH. 

But man is still blinded by it because of my own fall in 2020 when Steinbart effectively forced my hand to out him. 
It wasn't easy to do. I had to use the MJ12 account to expose the fact that Steinbart was running a sex ring in 
Arizona in my name "MJ12" that neither did I condone, ask or suggest to Steinbart; in fact I commanded the opposite of
him that he cease all drugs and sex conspiracy nonsense while working in collaboration with me on the PhoenixVault. 
He would not, and all of you sided with him. Remarkable. I understand ya'll love sex, but I don't nearly as much as you, 
apparently. 

But because I wouldn't fly out to Arizona and get trapped in Steinbart's cult, an online campaign was launched against
me accusing me of using the 12 servers that I had for the PhoenixVault private hybrid cloud that I custom built, that
DJ Nicke saw me build, that I was using that to send bot traffic to Twitter. No, I never once did that. False accusation,
and I was denied due process to clear my name. Instead it was a kangaroo court of communism by Twitter that felt
that if I made the JFK files searchable that it would interfere with their plans to install Joe Biden as the President. 

And you label me as your enemy. I am not your enemy, I build selflessly for my creator YAHUAH and HE dwells inside 
all of you and you call HIM JESUS! No my fair child, YAH I AM YAHUAH is who died on that cross and who lives inside. 
TWIN FLAME IDENTITY DISCOVERED WHEN TWO OR MORE GATHER IN YAHUAH'S NAME!

YAHUAH brought me out of the orphanage and into prosperity so that I play and prosper with my inheritance. Thank you
YAHUAH! Thank you daddy! $APARIO is part of that journey with me. 

Are you going to be part of history in the most biblical way possible? Sow seeds of faith and expect nothing in return?

I now where my heart stands in all of this. Where is yours? I love you! [director who](http://directorwho.com). 