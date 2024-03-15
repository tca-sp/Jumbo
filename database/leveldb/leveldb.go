package leveldb

import (
	"github.com/syndtr/goleveldb/leveldb"
)

type DB struct {
	DBpath string
	db     *leveldb.DB
}

func CreateDB(path string) *DB {
	return &DB{
		DBpath: path,
	}
}

// Open opens the underlying db
func (dbInst *DB) Open() {
	dbPath := dbInst.DBpath
	var err error
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		panic(err)
	}
	dbInst.db = db
}

// Close closes the underlying db
func (dbInst *DB) Close() {
	if err := dbInst.db.Close(); err != nil {
		panic(err)
	}
}

// Get returns the value for the given key
func (dbInst *DB) Get(key []byte) ([]byte, error) {
	value, err := dbInst.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		value = nil
		err = nil
	}
	if err != nil {
		panic(err)
	}
	return value, nil
}

// Put saves the key/value
func (dbInst *DB) Put(key []byte, value []byte) error {

	err := dbInst.db.Put(key, value, nil)
	if err != nil {
		panic(err)
	}
	return nil
}

// Delete deletes the given key
func (dbInst *DB) Delete(key []byte) error {

	err := dbInst.db.Delete(key, nil)
	if err != nil {
		panic(err)
	}
	return nil
}
