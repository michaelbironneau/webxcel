package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"	
	"github.com/labstack/echo"
	"strconv"
	"fmt"
	"time"
)

//Group is a collection of users. A user may be belong to multiple groups.
type Group struct {
	gorm.Model
	Name string
}

//List is a DataFunc to list all groups
func (g Group) List(c echo.Context) (interface{}, error) {
	db := c.Get("db").(gorm.DB)
	var groups []Group
	if err := db.Find(&groups).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"Groups": groups}, nil
}

//Get is a DataFunc to retrieve a single group
func (g Group) Get(c echo.Context) (interface{}, error){
	db := c.Get("db").(gorm.DB)
	id := c.Param("group_id")
	iid, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("Invalid user id")
	}
	var group Group 
	if err := db.First(&group, uint(iid)).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"Group": group}, nil
}

//User is a user of the system
type User struct {
	gorm.Model
	Login     string
	Passhash  string
	IsAdmin   bool
	IsAnalyst bool
	Groups    []Group `gorm:"many2many:user_group"`
}

//List is a DataFunc to list all groups
func (g User) List(c echo.Context) (interface{}, error) {
	db := c.Get("db").(gorm.DB)
	var users []User
	if err := db.Find(&users).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"Users": users}, nil
}

//Get is a DataFunc to retrieve a single group
func (g User) Get(c echo.Context) (interface{}, error){
	db := c.Get("db").(gorm.DB)
	id := c.Param("user_id")
	iid, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("Invalid user id")
	}
	u, _ := getCurrentUser(c)
	if !(u.IsAdmin || u.ID == uint(iid)) {
		return nil, fmt.Errorf("Unauthorized")
	}
	var user User
	if err := db.First(&user, uint(iid)).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"User": user}, nil
}

//Connection is a connection to an SQL database
type Connection struct {
	gorm.Model
	Name             string
	Description      string
	Driver           string
	ConnectionString string
}

//List is a DataFunc to list all connection
func (g Connection) List(c echo.Context) (interface{}, error) {
	db := c.Get("db").(gorm.DB)
	var connections []Connection
	if err := db.Find(&connections).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"Connections": connections}, nil
}

//Get is a DataFunc to retrieve a single connection
func (g Connection) Get(c echo.Context) (interface{}, error){
	db := c.Get("db").(gorm.DB)
	id := c.Param("connection_id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	var connection Connection 
	if err := db.First(&connection, uid).Error; err != nil {
		return nil, err
	}
	return map[string]interface{}{"Connection": connection}, nil
}


//Template is a report template.
type Template struct {
	gorm.Model
	Name        string
	Description string
	Group       Group
	Script      string `sql:"type:text"`
}

//List is a DataFunc to list all templates
func (g Template) List(c echo.Context) (interface{}, error) {
	db := c.Get("db").(gorm.DB)
	user, _ := getCurrentUser(c)
	var templates []Template
	var groups []Group
	if err := db.Model(&user).Related(&groups).Error; err != nil {
		return nil, err
	}
	
	for i := range groups {
		var template Template
		if !db.Model(&groups[i]).Related(&template).RecordNotFound(){
			templates = append(templates, template)
		}
	}
		
	return map[string]interface{}{"Templates": templates}, nil
}

//Get is a DataFunc to retrieve a single connection
func (g Template) Get(c echo.Context) (interface{}, error){
	db := c.Get("db").(gorm.DB)
	user, _ := getCurrentUser(c)
	id := c.Param("template_id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	var template Template 
	if err := db.First(&template, uint(uid)).Error; err != nil {
		return nil, err
	}
	var found bool 
	for i := range user.Groups {
		if user.Groups[i].ID == template.ID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("User not authorized to view template")
	}
	return map[string]interface{}{"Template": template}, nil
}

//ReportProgress is the progress of a report that is currently being generated. When the report
//has been generated Progress will be 100 and Message will be "Successfully generated" (or something
//to that effect).
type ReportProgress struct {
	ID       int
	Progress float64
	Message  string
}

//Report is a generated report.
type Report struct {
	gorm.Model
	Filename  string
	Template  Template
	CreatedBy User
	Content   string `sql:"type:text"` //base64 encoded content
	Status    ReportProgress
}

type ReportListItem struct {
	ReportID uint 
	Filename string 
	CreatedBy string 
	CreatedAt time.Time 
}

//List is a DataFunc to list all reports for a given template
func (g Report) List(c echo.Context) (interface{}, error) {
	db := c.Get("db").(gorm.DB)
	user, _ := getCurrentUser(c)
	id := c.Param("template_id")
	uid, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	rows, err := db.Raw(`
		SELECT r.id, r.filename, u.login, r.created_at FROM dbo.users l
			LEFT JOIN dbo.user_group g ON g.user_id = l.id  
			LEFT JOIN dbo.templates t ON t.group_id = g.group_id 
			LEFT JOIN dbo.reports r ON r.template_id = t.id 
			LEFT JOIN dbo.users u ON u.id = r.created_by
		WHERE l.id = ?
	`, user.ID).Rows()
	if err != nil {
		return nil, err
	}
	
	var ret []ReportListItem
	defer rows.Close()
	for rows.Next() {
		var rli ReportListItem
		rows.Scan(&rli.ReportID, &rli.Filename, &rli.CreatedBy, &rli.CreatedAt)
		ret = append(ret, rli)
	}
		
	return map[string]interface{}{"Reports": ret}, nil
}

//autoMigrateDatabase creates the necessary tables and indexes in the database
func autoMigrateDatabase(db *gorm.DB) {
	db.AutoMigrate(&Group{}, &User{}, &Template{}, &ReportProgress{}, &Report{}, &Connection{})
	db.Model(&Report{}).AddIndex("ix_report_created_at", "template_id", "created_at")
}
