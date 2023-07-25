package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/jcelliott/lumber"
)

const Version = "1.0.1"

type (
	Logger interface {
		Fatal(string, ...interface{})
		Error(string, ...interface{})
		Warn(string, ...interface{})
		Info(string, ...interface{})
		Debug(string, ...interface{})
		Trace(string, ...interface{})
	}

	Driver struct {
		mutex   sync.Mutex
		mutexes map[string]*sync.Mutex
		dir     string
		log     Logger
	}
)

type Options struct {
	Logger
}

func New(dir string, options *Options) (*Driver, error) {
	dir = filepath.Clean(dir)

	opts := Options{}
	if options != nil {
		opts = *options
	}

	if opts.Logger == nil {
		opts.Logger = lumber.NewConsoleLogger(lumber.INFO)
	}

	driver := Driver{
		dir:     dir,
		mutexes: make(map[string]*sync.Mutex),
		log:     opts.Logger,
	}

	if _, err := os.Stat(dir); err == nil {
		opts.Logger.Debug("using %s (database already exists)\n", dir)
	}

	opts.Logger.Debug("Creating the database at %s...\n", dir)

	return &driver, os.MkdirAll(dir, 0755)
}

func (d *Driver) Write(collection string, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("Missing Collection - no place to save the record!")
	}
	if resource == "" {
		return fmt.Errorf("Missing resource - unable to save record")
	}
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, collection)
	finalPath := filepath.Join(dir, resource+".json")
	tempPath := finalPath + ".tmp"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := json.MarshalIndent(v, "", "\t")
	if err != nil {
		return err
	}

	b = append(b, byte('\n'))
	if err := ioutil.WriteFile(tempPath, b, 0644); err != nil {
		return err
	}

	return os.Rename(tempPath, finalPath)
}

func (d *Driver) Read(collection, resource string, v interface{}) error {
	if collection == "" {
		return fmt.Errorf("Missing collection - unable to read")
	}
	if resource == "" {
		return fmt.Errorf("Missing resource - unable to save record (no name)!")
	}

	record := filepath.Join(d.dir, collection, resource)

	if _, err := stat(record); err != nil {
		return err
	}

	b, err := ioutil.ReadFile(record + ".json")
	if err != nil {
		return err
	}

	return json.Unmarshal(b, &v)
}

func (d *Driver) ReadAll(collection string) ([]string, error) {
	if collection == "" {
		return nil, fmt.Errorf("Missing collection - unable to read")
	}

	dir := filepath.Join(d.dir, collection)

	if _, err := stat(dir); err != nil {
		return nil, err
	}

	files, _ := ioutil.ReadDir(dir)

	var records []string
	for _, file := range files {
		b, err := ioutil.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, string(b))
	}

	return records, nil
}

func (d *Driver) Delete(collection, resource string) error {
	if collection == "" {
		return fmt.Errorf("Missing collection - unable to delete")
	}
	path := filepath.Join(collection, resource)
	mutex := d.getOrCreateMutex(collection)
	mutex.Lock()
	defer mutex.Unlock()

	dir := filepath.Join(d.dir, path)

	switch fi, err := stat(dir); {
	case fi == nil, err != nil:
		return fmt.Errorf("Unable to find file or directory named %v\n", path)

	case fi.Mode().IsDir():
		return os.RemoveAll(dir)

	case fi.Mode().IsRegular():
		return os.RemoveAll(dir + ".json")
	}

	return nil
}

func (d *Driver) getOrCreateMutex(collection string) *sync.Mutex {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	m, ok := d.mutexes[collection]

	if !ok {
		m = &sync.Mutex{}
		d.mutexes[collection] = m
	}
	return m
}

func stat(path string) (fi os.FileInfo, err error) {
	if fi, err = os.Stat(path); os.IsNotExist(err) {
		fi, err = os.Stat(path + ".json")
	}

	return
}

type User struct {
	Name    string
	Age     json.Number
	Contact string
	Company string
	Address Address
}

type Address struct {
	City    string
	State   string
	Country string
	Pincode json.Number
}

func main() {
	dir := "./"
	db, err := New(dir, nil)
	if err != nil {
		fmt.Println("Error", err)
	}

	employees := []User{
		User{
			"John", "23", "12345673", "Fr.CRCE", Address{"Mumbai", "Maharashtra", "India", "123456"},
		},
		User{
			"Paul", "25", "121515151", "Google", Address{"Banaglore", "Karnataka", "India", "1151351"},
		},
		User{
			"Robert", "49", "140811089", "Microsoft", Address{"Bangalore", "Karnataka", "India", "1212433"},
		},
		User{
			"Neo", "45", "1231241415", "Meta", Address{"Delhi", "Delhi", "India", "123151"},
		},
	}

	for _, value := range employees {
		db.Write("users", value.Name, User{
			Name:    value.Name,
			Age:     value.Age,
			Contact: value.Contact,
			Company: value.Company,
			Address: value.Address,
		})
	}

	records, err := db.ReadAll("users")
	if err != nil {
		fmt.Println("error", err)
	}
	fmt.Println(records)
	allUsers := []User{}

	for _, f := range records {
		employeeFound := User{}
		if err := json.Unmarshal([]byte(f), &employeeFound); err != nil {
			fmt.Println("error", err)
		}
		allUsers = append(allUsers, employeeFound)
	}
	fmt.Println(allUsers)

	if err := db.Delete("users", "John"); err != nil {
		fmt.Println("Error", err)
	}

	if err := db.Delete("users", ""); err != nil {
		fmt.Println("error", err)
	}

}
