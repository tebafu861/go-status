package main

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Site struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	URL         string    `json:"url"`
	Status      string    `json:"status"`
	LastChecked time.Time `json:"last_checked"`
}

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("sites.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	db.AutoMigrate(&Site{})
}

func main() {
	r := gin.Default()

	// Configure CORS
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	r.POST("/add_site", addSite)
	r.GET("/get_sites", getSites)
	r.GET("/get_site/:id", getSite)
	r.POST("/check_site/:id", checkSite)

	go checkSitesPeriodically()

	// Serve index.html for the root route
	r.StaticFile("/", "index.html")

	r.Run(":8000")
}

func addSite(c *gin.Context) {
	var site Site
	if err := c.ShouldBindJSON(&site); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	site.Status = "Unknown"
	db.Create(&site)
	c.JSON(http.StatusOK, gin.H{"message": "Site added!"})
}

func getSites(c *gin.Context) {
	var sites []Site
	db.Find(&sites)
	c.JSON(http.StatusOK, sites)
}

func getSite(c *gin.Context) {
	id := c.Param("id")
	var site Site
	if err := db.First(&site, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found"})
		return
	}
	c.JSON(http.StatusOK, site)
}

func checkSite(c *gin.Context) {
	id := c.Param("id")
	var site Site
	if err := db.First(&site, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Site not found"})
		return
	}

	updateSiteStatus(&site)
	db.Save(&site)
	c.JSON(http.StatusOK, gin.H{"message": "Site status updated", "site": site})
}

func updateSiteStatus(site *Site) {
	resp, err := http.Get("http://" + site.URL + "/")
	if err != nil {
		site.Status = "Down"
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			site.Status = "Down"
		} else if strings.Contains(string(body), `<img src="mdes.jpg" width="800" height="700">`) {
			site.Status = "Blocked"
		} else {
			site.Status = "Up"
		}
	}
	site.LastChecked = time.Now()
}

func checkSites() {
	var sites []Site
	db.Find(&sites)
	for _, site := range sites {
		if time.Since(site.LastChecked) >= 5*time.Minute {
			updateSiteStatus(&site)
			db.Save(&site)
		}
	}
}

func checkSitesPeriodically() {
	for {
		checkSites()
		time.Sleep(1 * time.Minute)
	}
}
