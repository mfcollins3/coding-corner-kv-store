package kvstore

import "os"

var openRead = os.Open
var openFile = os.OpenFile
var statFile = os.Stat
