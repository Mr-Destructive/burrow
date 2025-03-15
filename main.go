package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	models "github.com/mr-destructive/burrow/models"
	"github.com/mr-destructive/burrow/plugins"
	"gopkg.in/yaml.v3"

	"github.com/yuin/goldmark"
)

func WalkAndListFiles(dirPath string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return files, err
	}
	return files, nil
}

func ReadFiles(files []string) ([][]byte, error) {
	var filesBytes = [][]byte{}
	for _, file := range files {
		fileBytes, err := os.ReadFile(file)
		if err != nil {
			return filesBytes, err
		}
		filesBytes = append(filesBytes, fileBytes)
	}
	return filesBytes, nil
}

func ReadPosts(files []string) ([]models.Post, error) {
	var posts []models.Post

	// Read file contents
	filesBytes, err := ReadFiles(files)
	if err != nil {
		return nil, err
	}

	// Iterate through files
	for _, fileBytes := range filesBytes {
		var success bool
		var frontmatterObj models.FrontMatter
		var contentBytes []byte
		var requiredFields []string = []string{"title", "description", "status", "type", "date", "slug", "tags"}

		// Attempt to detect JSON front matter
		jsonSeparator := []byte("}\n\n")
		jsonIndex := strings.Index(string(fileBytes), string(jsonSeparator))

		if jsonIndex != -1 {
			frontmatterBytes := fileBytes[:jsonIndex+1] // Keep closing brace
			contentBytes = fileBytes[jsonIndex+2:]      // Skip the separator

			// Unmarshal into a temporary map to capture extra fields
			tempMap := make(map[string]interface{})
			if err := json.Unmarshal(frontmatterBytes, &tempMap); err == nil {
				success = true

				// Extract known fields into the struct
				if err := json.Unmarshal(frontmatterBytes, &frontmatterObj); err != nil {
					log.Printf("Error parsing JSON front matter: %v", err)
					continue
				}

				// Remove known keys and store the rest in Extras
				for _, key := range requiredFields {
					delete(tempMap, key)
				}
				frontmatterObj.Extras = tempMap
			}
		}

		// Attempt to detect YAML front matter
		if !success {
			yamlSeparator := []byte("---\n\n")
			yamlIndex := strings.Index(string(fileBytes), string(yamlSeparator))

			if yamlIndex != -1 {
				frontmatterBytes := fileBytes[:yamlIndex]
				contentBytes = fileBytes[yamlIndex+len(yamlSeparator):]

				// Unmarshal into a temporary map to capture extra fields
				tempMap := make(map[string]interface{})
				if err := yaml.Unmarshal(frontmatterBytes, &tempMap); err == nil {

					// Extract known fields into the struct
					if err := yaml.Unmarshal(frontmatterBytes, &frontmatterObj); err != nil {
						log.Printf("Error parsing YAML front matter: %v", err)
						continue
					}

					// Remove known keys and store the rest in Extras
					for _, key := range requiredFields {
						delete(tempMap, key)
					}
					frontmatterObj.Extras = tempMap
				} else {
					log.Printf("Error parsing YAML front matter: %v", err)
					continue
				}
			} else {
				log.Printf("No valid front matter found in file")
				continue
			}
		}
		// Convert Markdown to HTML
		var contentBuffer bytes.Buffer
		if err := goldmark.Convert(contentBytes, &contentBuffer); err != nil {
			log.Printf("Error processing Markdown: %v", err)
			continue
		}

		// Append post
		posts = append(posts, models.Post{
			Frontmatter: frontmatterObj,
			Content:     template.HTML(contentBuffer.String()),
		})
	}

	return posts, nil
}

func ReadTemplates(files []string) ([]string, error) {
	templateStrs := []string{}
	filesBytes, err := ReadFiles(files)
	if err != nil {
		return templateStrs, err
	}
	for _, fileBytes := range filesBytes {
		templateStrs = append(templateStrs, string(fileBytes))
	}
	return templateStrs, nil
}

func Copy(src string, dst string) error {
	srcFiles, err := WalkAndListFiles(src)
	if err != nil {
		return err
	}
	for _, srcFileName := range srcFiles {
		srcFile, err := os.Open(srcFileName)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		srcFileName = filepath.Base(srcFileName)
		dstPath := filepath.Join(dst, srcFileName)
		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			log.Fatal(err)
		}
	}
	return nil
}

type PluginManager struct {
	Plugins []plugins.Plugin
}

func (p *PluginManager) Register(plugin plugins.Plugin) {
	p.Plugins = append(p.Plugins, plugin)
}

func (p *PluginManager) ExecuteAll(ssg *models.SSG) {
	fmt.Println("Running plugins")
	for _, plugin := range p.Plugins {
		fmt.Println("Running plugin:", plugin.Name())
		plugin.Execute(ssg)
	}
}

// Config Plugin
type ConfigPlugin struct {
	PluginName string
}

func (c *ConfigPlugin) Name() string {
	return c.PluginName
}

type PostReaderPlugin struct {
	PluginName string
}

func (c *PostReaderPlugin) Name() string {
	return c.PluginName
}

func (c *PostReaderPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config
	postFolder := config.Blog.PostsDir
	var postFiles []string
	postFiles, err := WalkAndListFiles(postFolder)
	if err != nil {
		log.Fatal(err)
	}
	postsList, err := ReadPosts(postFiles)
	if err != nil {
		log.Fatal(err)
	}
	for _, post := range postsList {
		if post.Frontmatter.Status == "draft" {
			continue
		}
		plugins.CleanPostFrontmatter(&post, ssg)
	}
	ssg.Posts = postsList
}

type RenderTemplatesPlugin struct {
	PluginName string
}

func (c *RenderTemplatesPlugin) Name() string {
	return c.PluginName
}

func (c *RenderTemplatesPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config
	templateFS := os.DirFS(config.Blog.TemplatesDir)
	ssg.FS = templateFS
	t, err := template.ParseFS(templateFS, "*.html")
	ssg.TemplateFS = t
	if err != nil {
		log.Fatal(err)
	}
	var prefixURL string = ""
	if config.Blog.PrefixURL != "" {
		prefixURL = config.Blog.PrefixURL
	}

	// render the templates with the content
	outputPath := filepath.Join(".", config.Blog.OutputDir)
	err = os.MkdirAll(outputPath, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	feedPosts := make(map[string][]models.Post)

	for _, post := range ssg.Posts {
		if post.Frontmatter.Status == "draft" {
			continue
		}
		postType := post.Frontmatter.Type
		if postType == "" {
			postType = "posts"
		}
		templatePath := config.Blog.PagesConfig[postType].TemplatePath
		if templatePath == "" {
			templatePath = config.Blog.DefaultPostTemplate
		}
		buffer := bytes.Buffer{}
		postSlug := post.Frontmatter.Slug
		if postSlug == "" {
			postSlug = plugins.Slugify(post.Frontmatter.Title)
		}
		post.Frontmatter.Slug = prefixURL + postType + "/" + postSlug
		postPath := filepath.Join(outputPath, postType, postSlug)
		//outputDirPath := filepath.Join(postPath, postSlug)
		err = os.MkdirAll(postPath, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
		outputPostPath := filepath.Join(postPath, "index.html")
		//sort post by date
		sort.Slice(ssg.Posts, func(i, j int) bool {
			// Ensure correct parsing and handle errors
			date1, err1 := time.Parse("2006-01-02", ssg.Posts[i].Frontmatter.Date)
			date2, err2 := time.Parse("2006-01-02", ssg.Posts[j].Frontmatter.Date)

			if err1 != nil {
				date1 = time.Time{}
			}
			if err2 != nil {
				date2 = time.Time{}
			}

			return date2.Before(date1)
		})

		for i := range ssg.Posts {
			ssg.Posts[i].Frontmatter.Date = ssg.Posts[i].Frontmatter.Date[:10] // Truncate in case of time component
		}
		context := models.TemplateContext{
			Post: post,
			Themes: models.ThemeCombo{
				Default:   config.Blog.Themes["default"],
				Secondary: config.Blog.Themes["secondary"],
			},
			Config: models.SSG_CONFIG{
				Blog: config.Blog,
			},
		}
		err := t.ExecuteTemplate(&buffer, templatePath, context)
		if err != nil {
			log.Fatal(err)
		}
		err = os.WriteFile(outputPostPath, buffer.Bytes(), 0660)
		feedPosts[postType] = append(feedPosts[postType], post)
	}
	for postType, posts := range feedPosts {
		fmt.Println(postType)
		fmt.Println(len(posts))
	}
	feedPostLists := []models.Feed{}
	for postType, posts := range feedPosts {
		fmt.Println(config.Blog.PagesConfig[postType].Emoji)
		feed := models.Feed{
			Title: strings.ToTitle(postType) + " " + config.Blog.PagesConfig[postType].Emoji,
			Type:  postType,
			Slug:  prefixURL + postType,
			Posts: posts,
		}
		feedPostLists = append(feedPostLists, feed)
	}
	ssg.FeedPosts = feedPostLists
}

// "createFeeds",
type CreateFeedsPlugin struct {
	PluginName string
}

func (c *CreateFeedsPlugin) Name() string {
	return c.PluginName
}

func (c *CreateFeedsPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config
	for _, feed := range ssg.FeedPosts {
		buffer := bytes.Buffer{}
		templatePath := config.Blog.PagesConfig[feed.Type].FeedTemplatePath
		if templatePath == "" {
			templatePath = config.Blog.DefaultFeedTemplate
		}

		context := models.TemplateContext{
			FeedPosts: []models.Feed{feed},
			Themes: models.ThemeCombo{
				Default:   config.Blog.Themes["default"],
				Secondary: config.Blog.Themes["secondary"],
			},
			FeedInfo: feed,
			Config: models.SSG_CONFIG{
				Blog: config.Blog,
			},
		}
		err := ssg.TemplateFS.ExecuteTemplate(&buffer, templatePath, context)
		if err != nil {
			log.Fatal(err)
		}
		feedPath := filepath.Join(".", config.Blog.OutputDir, feed.Type)
		err = os.MkdirAll(feedPath, os.ModePerm)
		outputFeedPath := fmt.Sprintf("%s/index.html", feedPath)
		err = os.WriteFile(outputFeedPath, buffer.Bytes(), 0660)
	}
}

// "createFeeds",
// "copyStaticFiles",
type CopyStaticFilesPlugin struct {
	PluginName string
}

func (c *CopyStaticFilesPlugin) Name() string {
	return c.PluginName
}

func (c *CopyStaticFilesPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config
	err := Copy(config.Blog.StaticDir, config.Blog.OutputDir)
	if err != nil {
		log.Fatal(err)
	}
}

type IndexPlugin struct {
	PluginName string
}

func (c *IndexPlugin) Name() string {
	return c.PluginName
}

func (c *IndexPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config

	buffer := bytes.Buffer{}
	templateFS := os.DirFS(config.Blog.StaticDir)
	t, err := template.ParseFS(templateFS, "index.html")
	if err != nil {
		log.Fatal(err)
	}
	context := models.TemplateContext{
		Themes: models.ThemeCombo{
			Default:   config.Blog.Themes["default"],
			Secondary: config.Blog.Themes["secondary"],
		},
		Config: models.SSG_CONFIG{
			Blog: config.Blog,
		},
		FeedPosts: ssg.FeedPosts,
	}
	err = t.ExecuteTemplate(&buffer, "index.html", context)
	if err != nil {
		log.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(".", config.Blog.OutputDir, "index.html"), buffer.Bytes(), 0660)
	if err != nil {
		log.Fatal(err)
	}
}

// "server"
type ServerPlugin struct {
	PluginName string
}

func (c *ServerPlugin) Name() string {
	return c.PluginName
}

func (c *ServerPlugin) Execute(ssg *models.SSG) {
	config := &ssg.Config
	http.Handle("/", http.FileServer(http.Dir(config.Blog.OutputDir)))
	fmt.Println("Listening on port 3030")
	http.ListenAndServe(":3030", nil)
}

func main() {

	args := os.Args
	devEnv := false
	if len(args) > 1 {
		if args[1] == "dev" {
			devEnv = true
		}
	}
	// read the config
	// read all the plugins from config file
	ssg := models.SSG{}
	configbytes, err := os.ReadFile(models.SSG_CONFIG_FILE_NAME)
	if err != nil {
		log.Fatal(err)
	}
	var config models.SSG_CONFIG
	err = json.Unmarshal(configbytes, &config)
	if err != nil {
		log.Fatal(err)
	}
	ssg.Config = config
	if devEnv {
		ssg.Config.Blog.PrefixURL = ""
	}

	// loading in the posts -> post folder
	// load in the templates
	// create feeds
	// load in the static files
	// pack the html pages and static files in a folder
	// serve the folder
	pluginManager := PluginManager{}
	for _, plugin := range config.Plugins {
		switch plugin {
		case "readPosts":
			pluginManager.Register(&PostReaderPlugin{PluginName: "readPosts"})
		case "renderTemplates":
			pluginManager.Register(&RenderTemplatesPlugin{PluginName: "renderTemplates"})
		case "createFeeds":
			pluginManager.Register(&CreateFeedsPlugin{PluginName: "createFeeds"})
		case "copyStaticFiles":
			pluginManager.Register(&CopyStaticFilesPlugin{PluginName: "copyStaticFiles"})
		case "index":
			pluginManager.Register(&IndexPlugin{PluginName: "index"})
		default:

			//userPlugin := plugins.UserPlugin{PluginName: plugin}
			//pluginManager.Register(&userPlugin)

			if plugin != "server" {
				pluginStruct, err := LoadPlugin(plugin)
				if err != nil {
					log.Printf("Error loading plugin %s: %v", plugin, err)
					continue
				}
				fmt.Println("Load", plugin, pluginStruct)
				pluginManager.Register(pluginStruct)
			} else {
				continue
			}
		}
	}
	pluginManager.ExecuteAll(&ssg)
	pluginManager = PluginManager{}
	if devEnv {
		pluginManager.Register(&ServerPlugin{PluginName: "server"})
	}
	pluginManager.ExecuteAll(&ssg)
}

func LoadPlugin(pluginName string) (plugins.Plugin, error) {
	pluginType, exists := plugins.GetPluginType(pluginName)
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginName)
	}

	pluginValue := reflect.New(pluginType).Interface()
	pluginInstance, ok := pluginValue.(plugins.Plugin)
	if !ok {
		return nil, fmt.Errorf("type %s does not implement Plugin interface", pluginName)
	}
	val := reflect.ValueOf(pluginInstance).Elem()
	if field := val.FieldByName("PluginName"); field.IsValid() && field.CanSet() {
		field.SetString(pluginName)
	}
	return pluginInstance, nil
}
