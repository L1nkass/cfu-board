package middleware

import "time"

// DbOpTimeout — таймаут операций с базой данных.
const DbOpTimeout = 5 * time.Second

// DefaultPostsLimit — размер страницы ленты по умолчанию.
const DefaultPostsLimit = 20

// MaxPostsLimit — жёсткий потолок limit на запрос.
const MaxPostsLimit = 100
