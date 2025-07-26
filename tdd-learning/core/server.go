// server.go
package core

import (
	"encoding/json"
	"net/http"
)

// CacheServer HTTP缓存服务器
type CacheServer struct {
    cache *LRUCache
    mux   *http.ServeMux
}

// NewCacheServer 创建新的缓存服务器
func NewCacheServer(cache *LRUCache) *CacheServer {
    // 实现构造函数
    // 注册路由：/cache, /cache/, /stats
    mux := http.NewServeMux()
	s := &CacheServer{
		cache: cache,
		mux:   mux,
	}
	mux.HandleFunc("/cache", s.handleCache)
    mux.HandleFunc("/cache/", s.handleCacheWithKey)
    mux.HandleFunc("/stats", s.handleStats)
    return s
}

// ServeHTTP 实现http.Handler接口
func (s *CacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 委托给mux处理
	s.mux.ServeHTTP(w, r)
}

// handleCache 处理 POST /cache
func (s *CacheServer) handleCache(w http.ResponseWriter, r *http.Request) {
    // 只处理POST请求
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
    // 解析JSON: {"key": "...", "value": "..."}
	var content map[string]any
	err := json.NewDecoder(r.Body).Decode(&content)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	// 安全的类型断言
	// key, value := content["key"].(string), content["value"].(string)
	key, keyOk := content["key"].(string)
	value, valueOk := content["value"].(string)
	if !keyOk || !valueOk {
		http.Error(w, "Missing key or value", http.StatusBadRequest)
		return
	}
	// 调用cache.Set()
	s.cache.Set(key, value)
    // 返回JSON响应
	w.WriteHeader(http.StatusOK)
}

// handleCacheWithKey 处理 GET/DELETE /cache/{key} - 传入key的方法处理
func (s *CacheServer) handleCacheWithKey(w http.ResponseWriter, r *http.Request) {
	// 提取key: /cache/mykey -> mykey
	key := r.URL.Path[len("/cache/"):]
    // GET: 调用cache.Get(), 返回值或404
	if r.Method == http.MethodGet {
		value, exists := s.cache.Get(key)
		if exists {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			response := map[string]string {
				"key" : key,
				"value" : value,
			}
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Key not found", http.StatusNotFound)
		}
	}
	// DELETE: 调用cache.Delete(), 返回结果
	if r.Method == http.MethodDelete {
		ok := s.cache.Delete(key)
		if !ok {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
}

// handleStats 处理 GET /stats
func (s *CacheServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// 获取cache.GetStats()
		s.cache.GetStats()
		// 返回JSON格式的统计信息
		response := s.buildStatsResponse()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// json 无法格式化的类型：interface complex 和 chan func unsafe.pointer