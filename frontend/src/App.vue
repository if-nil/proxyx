<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'

interface MySQLEvent {
  type: string
  query: string
  args?: any[]
  database: string
  timestamp: string
  duration: number
  error?: string
  row_count: number
}

interface RedisEvent {
  command: string
  args: string[]
  raw: string
  timestamp: string
  duration: number
  error?: string
  response: string
}

interface Message {
  type: 'mysql' | 'redis'
  data: MySQLEvent | RedisEvent
}

const mysqlEvents = ref<MySQLEvent[]>([])
const redisEvents = ref<RedisEvent[]>([])
const activeTab = ref<'all' | 'mysql' | 'redis'>('all')
const connected = ref(false)
const searchQuery = ref('')
const isPaused = ref(false)

let ws: WebSocket | null = null

const filteredMySQLEvents = computed(() => {
  if (!searchQuery.value) return mysqlEvents.value
  const query = searchQuery.value.toLowerCase()
  return mysqlEvents.value.filter(e => 
    e.query.toLowerCase().includes(query) || 
    e.database.toLowerCase().includes(query)
  )
})

const filteredRedisEvents = computed(() => {
  if (!searchQuery.value) return redisEvents.value
  const query = searchQuery.value.toLowerCase()
  return redisEvents.value.filter(e => 
    e.command.toLowerCase().includes(query) || 
    (e.args && e.args.some(a => a.toLowerCase().includes(query)))
  )
})

function formatDuration(ns: number): string {
  if (ns < 1000) return `${ns}ns`
  if (ns < 1000000) return `${(ns / 1000).toFixed(2)}µs`
  if (ns < 1000000000) return `${(ns / 1000000).toFixed(2)}ms`
  return `${(ns / 1000000000).toFixed(2)}s`
}

function formatTime(timestamp: string): string {
  const date = new Date(timestamp)
  const time = date.toLocaleTimeString('zh-CN', { 
    hour: '2-digit', 
    minute: '2-digit', 
    second: '2-digit'
  })
  const ms = date.getMilliseconds().toString().padStart(3, '0')
  return `${time}.${ms}`
}

function connect() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws`
  
  ws = new WebSocket(wsUrl)
  
  ws.onopen = () => {
    connected.value = true
    console.log('WebSocket connected')
  }
  
  ws.onmessage = (event) => {
    if (isPaused.value) return
    
    const msg: Message = JSON.parse(event.data)
    
    if (msg.type === 'mysql') {
      mysqlEvents.value.unshift(msg.data as MySQLEvent)
      if (mysqlEvents.value.length > 500) {
        mysqlEvents.value.pop()
      }
    } else if (msg.type === 'redis') {
      redisEvents.value.unshift(msg.data as RedisEvent)
      if (redisEvents.value.length > 500) {
        redisEvents.value.pop()
      }
    }
  }
  
  ws.onclose = () => {
    connected.value = false
    console.log('WebSocket disconnected, reconnecting...')
    setTimeout(connect, 2000)
  }
  
  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }
}

function clearAll() {
  mysqlEvents.value = []
  redisEvents.value = []
}

function togglePause() {
  isPaused.value = !isPaused.value
}

onMounted(() => {
  connect()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
  }
})
</script>

<template>
  <div class="app">
    <!-- Header -->
    <header class="header">
      <div class="header-left">
        <h1 class="logo">
          <span class="logo-icon">⚡</span>
          ProxyX Monitor
        </h1>
        <div class="status" :class="{ connected }">
          <span class="status-dot"></span>
          {{ connected ? '已连接' : '连接中...' }}
        </div>
      </div>
      <div class="header-right">
        <div class="search-box">
          <input 
            v-model="searchQuery" 
            type="text" 
            placeholder="搜索命令..." 
            class="search-input"
          />
        </div>
        <button class="btn" :class="{ active: isPaused }" @click="togglePause">
          {{ isPaused ? '▶ 继续' : '⏸ 暂停' }}
        </button>
        <button class="btn btn-danger" @click="clearAll">
          🗑 清空
        </button>
      </div>
    </header>

    <!-- Tabs -->
    <nav class="tabs">
      <button 
        class="tab" 
        :class="{ active: activeTab === 'all' }"
        @click="activeTab = 'all'"
      >
        全部
        <span class="badge">{{ mysqlEvents.length + redisEvents.length }}</span>
      </button>
      <button 
        class="tab tab-mysql" 
        :class="{ active: activeTab === 'mysql' }"
        @click="activeTab = 'mysql'"
      >
        MySQL
        <span class="badge mysql">{{ mysqlEvents.length }}</span>
      </button>
      <button 
        class="tab tab-redis" 
        :class="{ active: activeTab === 'redis' }"
        @click="activeTab = 'redis'"
      >
        Redis
        <span class="badge redis">{{ redisEvents.length }}</span>
      </button>
    </nav>

    <!-- Content -->
    <main class="content">
      <!-- MySQL Events -->
      <section 
        v-if="activeTab === 'all' || activeTab === 'mysql'" 
        class="events-section"
      >
        <h2 v-if="activeTab === 'all'" class="section-title mysql">
          <span class="icon">🐬</span> MySQL Queries
        </h2>
        <div class="events-list">
          <TransitionGroup name="list">
            <div 
              v-for="(event, index) in filteredMySQLEvents" 
              :key="`mysql-${index}-${event.timestamp}`"
              class="event-card mysql"
              :class="{ error: event.error }"
            >
              <div class="event-header">
                <span class="event-type">{{ event.type.toUpperCase() }}</span>
                <span class="event-db" v-if="event.database">{{ event.database }}</span>
                <span class="event-time">{{ formatTime(event.timestamp) }}</span>
                <span class="event-duration" :class="{ slow: event.duration > 100000000 }">
                  {{ formatDuration(event.duration) }}
                </span>
              </div>
              <div class="event-body">
                <code class="event-query">{{ event.query }}</code>
                <div v-if="event.args?.length" class="event-args">
                  Args: {{ JSON.stringify(event.args) }}
                </div>
              </div>
              <div class="event-footer">
                <span v-if="event.error" class="event-error">❌ {{ event.error }}</span>
                <span v-else class="event-success">✓ {{ event.row_count }} rows</span>
              </div>
            </div>
          </TransitionGroup>
          <div v-if="filteredMySQLEvents.length === 0" class="empty-state">
            暂无 MySQL 查询记录
          </div>
        </div>
      </section>

      <!-- Redis Events -->
      <section 
        v-if="activeTab === 'all' || activeTab === 'redis'" 
        class="events-section"
      >
        <h2 v-if="activeTab === 'all'" class="section-title redis">
          <span class="icon">🔴</span> Redis Commands
        </h2>
        <div class="events-list">
          <TransitionGroup name="list">
            <div 
              v-for="(event, index) in filteredRedisEvents" 
              :key="`redis-${index}-${event.timestamp}`"
              class="event-card redis"
              :class="{ error: event.error }"
            >
              <div class="event-header">
                <span class="event-type">{{ event.command }}</span>
                <span class="event-time">{{ formatTime(event.timestamp) }}</span>
                <span class="event-duration" :class="{ slow: event.duration > 10000000 }">
                  {{ formatDuration(event.duration) }}
                </span>
              </div>
              <div class="event-body">
                <code class="event-query">
                  {{ event.command }} {{ event.args?.join(' ') || '' }}
                </code>
              </div>
              <div class="event-footer">
                <span v-if="event.error" class="event-error">❌ {{ event.error }}</span>
                <span v-else class="event-response">→ {{ event.response }}</span>
              </div>
            </div>
          </TransitionGroup>
          <div v-if="filteredRedisEvents.length === 0" class="empty-state">
            暂无 Redis 命令记录
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* Header */
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 24px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 24px;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo {
  font-size: 1.5rem;
  font-weight: 700;
  display: flex;
  align-items: center;
  gap: 8px;
  background: linear-gradient(135deg, var(--accent-mysql), var(--accent-redis));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.logo-icon {
  font-size: 1.8rem;
  -webkit-text-fill-color: initial;
}

.status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--accent-error);
  animation: pulse 2s infinite;
}

.status.connected .status-dot {
  background: var(--accent-success);
  animation: none;
}

.search-box {
  position: relative;
}

.search-input {
  padding: 8px 16px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 0.875rem;
  width: 240px;
  outline: none;
  transition: border-color 0.2s;
}

.search-input:focus {
  border-color: var(--accent-mysql);
}

.btn {
  padding: 8px 16px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 0.875rem;
  cursor: pointer;
  transition: all 0.2s;
}

.btn:hover {
  background: var(--border-color);
}

.btn.active {
  background: var(--accent-mysql);
  border-color: var(--accent-mysql);
  color: var(--bg-primary);
}

.btn-danger:hover {
  background: var(--accent-error);
  border-color: var(--accent-error);
}

/* Tabs */
.tabs {
  display: flex;
  gap: 4px;
  padding: 12px 24px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
}

.tab {
  padding: 8px 20px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 0.9rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s;
}

.tab:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.tab.active {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.badge {
  padding: 2px 8px;
  border-radius: 12px;
  background: var(--border-color);
  font-size: 0.75rem;
  font-weight: 600;
}

.badge.mysql {
  background: rgba(0, 217, 255, 0.2);
  color: var(--accent-mysql);
}

.badge.redis {
  background: rgba(255, 107, 107, 0.2);
  color: var(--accent-redis);
}

/* Content */
.content {
  flex: 1;
  padding: 24px;
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(500px, 1fr));
  gap: 24px;
  align-items: start;
}

.events-section {
  background: var(--bg-secondary);
  border-radius: 12px;
  border: 1px solid var(--border-color);
  overflow: hidden;
}

.section-title {
  padding: 16px 20px;
  font-size: 1rem;
  font-weight: 600;
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-title.mysql {
  color: var(--accent-mysql);
}

.section-title.redis {
  color: var(--accent-redis);
}

.events-list {
  max-height: calc(100vh - 280px);
  overflow-y: auto;
  padding: 12px;
}

.event-card {
  background: var(--bg-tertiary);
  border-radius: 8px;
  padding: 12px 16px;
  margin-bottom: 8px;
  border-left: 3px solid;
  animation: slideIn 0.3s ease-out;
}

.event-card.mysql {
  border-left-color: var(--accent-mysql);
}

.event-card.redis {
  border-left-color: var(--accent-redis);
}

.event-card.error {
  border-left-color: var(--accent-error);
  background: rgba(248, 81, 73, 0.1);
}

.event-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.event-type {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.mysql .event-type {
  background: rgba(0, 217, 255, 0.2);
  color: var(--accent-mysql);
}

.redis .event-type {
  background: rgba(255, 107, 107, 0.2);
  color: var(--accent-redis);
}

.event-db {
  padding: 2px 8px;
  border-radius: 4px;
  background: var(--border-color);
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.event-time {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-left: auto;
}

.event-duration {
  padding: 2px 8px;
  border-radius: 4px;
  background: rgba(63, 185, 80, 0.2);
  color: var(--accent-success);
  font-size: 0.75rem;
  font-family: var(--font-mono);
}

.event-duration.slow {
  background: rgba(248, 81, 73, 0.2);
  color: var(--accent-error);
}

.event-body {
  margin-bottom: 8px;
}

.event-query {
  font-family: var(--font-mono);
  font-size: 0.875rem;
  color: var(--text-primary);
  word-break: break-all;
  white-space: pre-wrap;
}

.event-args {
  margin-top: 8px;
  font-size: 0.75rem;
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.event-footer {
  font-size: 0.75rem;
}

.event-success {
  color: var(--accent-success);
}

.event-error {
  color: var(--accent-error);
}

.event-response {
  color: var(--text-secondary);
  font-family: var(--font-mono);
}

.empty-state {
  text-align: center;
  padding: 48px 24px;
  color: var(--text-secondary);
}

/* Transitions */
.list-enter-active {
  animation: slideIn 0.3s ease-out;
}

.list-leave-active {
  animation: slideIn 0.3s ease-out reverse;
}

/* Responsive */
@media (max-width: 768px) {
  .header {
    flex-direction: column;
    gap: 16px;
  }
  
  .header-right {
    width: 100%;
    flex-wrap: wrap;
  }
  
  .search-input {
    width: 100%;
  }
  
  .content {
    grid-template-columns: 1fr;
  }
}
</style>

