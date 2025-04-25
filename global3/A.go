package global3

import "sync"

var GlobalStateDB interface{}

var Lock sync.Mutex