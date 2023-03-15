package stringcache

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/0xERR0R/blocky/log"
	"github.com/0xERR0R/blocky/redis"
	"github.com/hako/durafmt"
	"github.com/rueian/rueidis"
)

type StringCache interface {
	ElementCount() int
	Contains(searchString string) bool
}

type CacheFactory interface {
	AddEntry(entry string)
	Create() StringCache
}

type redisStringCache struct {
	rdb *redis.Client2
	key string
}

func (cache *redisStringCache) ElementCount() int {
	res, err := cache.rdb.Scard(cache.key)
	if err != nil {
		panic(err)
	}
	return int(res)
}

func (cache *redisStringCache) Contains(searchString string) bool {
	now := time.Now()
	found, err := cache.rdb.Sismember(cache.key, searchString)
	if err != nil {
		panic(err)
	}
	log.Log().Infof("redis lookup in set '%s', domain '%s': result: %t, duration %s", cache.key, searchString, found, durafmt.Parse(time.Since(now)).String())

	return found
}

type stringCache map[int]string

func normalizeEntry(entry string) string {
	return strings.ToLower(entry)
}

func (cache stringCache) ElementCount() int {
	count := 0

	for k, v := range cache {
		count += len(v) / k
	}

	return count
}

func (cache stringCache) Contains(searchString string) bool {
	normalized := normalizeEntry(searchString)
	searchLen := len(normalized)

	if searchLen == 0 {
		return false
	}

	searchBucketLen := len(cache[searchLen]) / searchLen
	idx := sort.Search(searchBucketLen, func(i int) bool {
		return cache[searchLen][i*searchLen:i*searchLen+searchLen] >= normalized
	})

	if idx < searchBucketLen {
		return cache[searchLen][idx*searchLen:idx*searchLen+searchLen] == strings.ToLower(normalized)
	}

	return false
}

type redisStringCacheFactory struct {
	rdb  *redis.Client2
	name string
	cmds rueidis.Commands
}

func newRedisStringCacheFactory(rdb *redis.Client2, name string) CacheFactory {
	cmds := rueidis.Commands{rdb.C().B().Del().Key(name).Build()}

	return &redisStringCacheFactory{
		name: name,
		rdb:  rdb,
		cmds: cmds,
	}
}

func (s *redisStringCacheFactory) AddEntry(entry string) {
	s.cmds = append(s.cmds, s.rdb.C().B().Sadd().Key(s.name).Member(entry).Build())
}

func (s *redisStringCacheFactory) Create() StringCache {
	s.rdb.DoMulti(s.cmds)
	return &redisStringCache{
		rdb: s.rdb,
		key: s.name,
	}
}

type stringCacheFactory struct {
	// temporary map which holds sorted slice of strings grouped by string length
	tmp map[int][]string
}

func newStringCacheFactory() CacheFactory {
	return &stringCacheFactory{
		tmp: make(map[int][]string),
	}
}

func (s *stringCacheFactory) getBucket(length int) []string {
	if s.tmp[length] == nil {
		s.tmp[length] = make([]string, 0)
	}

	return s.tmp[length]
}

func (s *stringCacheFactory) insertString(entry string) {
	normalized := normalizeEntry(entry)
	entryLen := len(normalized)
	bucket := s.getBucket(entryLen)
	ix := sort.SearchStrings(bucket, normalized)

	if !(ix < len(bucket) && bucket[ix] == normalized) {
		// extent internal bucket
		bucket = append(s.getBucket(entryLen), "")

		// move elements to make place for the insertion
		copy(bucket[ix+1:], bucket[ix:])

		// insert string at the calculated position
		bucket[ix] = normalized
		s.tmp[entryLen] = bucket
	}
}

func (s *stringCacheFactory) AddEntry(entry string) {
	// skip empty strings
	if len(entry) > 0 {
		s.insertString(entry)
	}
}

func (s *stringCacheFactory) Create() StringCache {
	cache := make(stringCache, len(s.tmp))
	for k, v := range s.tmp {
		cache[k] = strings.Join(v, "")
	}

	s.tmp = nil

	return cache
}

type regexCache []*regexp.Regexp

func (cache regexCache) ElementCount() int {
	return len(cache)
}

func (cache regexCache) Contains(searchString string) bool {
	for _, regex := range cache {
		if regex.MatchString(searchString) {
			log.PrefixedLog("regexCache").Debugf("regex '%s' matched with '%s'", regex, searchString)

			return true
		}
	}

	return false
}

type regexCacheFactory struct {
	cache regexCache
}

func (r *regexCacheFactory) AddEntry(entry string) {
	compile, err := regexp.Compile(entry)
	if err != nil {
		log.Log().Warnf("invalid regex '%s'", entry)
	} else {
		r.cache = append(r.cache, compile)
	}
}

func (r *regexCacheFactory) Create() StringCache {
	return r.cache
}

func newRegexCacheFactory() CacheFactory {
	return &regexCacheFactory{
		cache: make(regexCache, 0),
	}
}

type chainedCache struct {
	caches []StringCache
}

func (cache chainedCache) ElementCount() int {
	sum := 0
	for _, c := range cache.caches {
		sum += c.ElementCount()
	}

	return sum
}

func (cache chainedCache) Contains(searchString string) bool {
	for _, c := range cache.caches {
		if c.Contains(searchString) {
			return true
		}
	}

	return false
}

type chainedCacheFactory struct {
	stringCacheFactory CacheFactory
	regexCacheFactory  CacheFactory
}

func (r *chainedCacheFactory) AddEntry(entry string) {
	if strings.HasPrefix(entry, "/") && strings.HasSuffix(entry, "/") {
		entry = strings.TrimSpace(entry[1 : len(entry)-1])
		r.regexCacheFactory.AddEntry(entry)
	} else {
		r.stringCacheFactory.AddEntry(entry)
	}
}

func (r *chainedCacheFactory) Create() StringCache {
	return &chainedCache{
		caches: []StringCache{r.stringCacheFactory.Create(), r.regexCacheFactory.Create()},
	}
}

func NewChainedCacheFactory(name string) CacheFactory {
	return &chainedCacheFactory{
		stringCacheFactory: newRedisStringCacheFactory(redis.GetRedisClient(), name),
		regexCacheFactory:  newRegexCacheFactory(),
	}
}
