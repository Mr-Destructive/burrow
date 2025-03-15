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
	filesBytes, err := ReadFiles(files)
	if err != nil {
		return posts, err
	}
	for _, fileBytes := range filesBytes {
		frontmatterSeparator := []byte("}\n\n")
		index := strings.Index(string(fileBytes), string(frontmatterSeparator))
		frontmatterBytes := fileBytes[:index+len(frontmatterSeparator)]
		contentBytes := fileBytes[index+len(frontmatterSeparator):]
		var frontmatterObj models.FrontMatter

		err = json.Unmarshal(frontmatterBytes, &frontmatterObj)
		if err != nil {
			frontmatterSeparator = []byte("---\n\n")
			index = strings.Index(string(fileBytes), string(frontmatterSeparator))
			if index == -1 {
				continue
			}
			frontmatterBytes = fileBytes[:index+len(frontmatterSeparator)]
			contentBytes = fileBytes[index+len(frontmatterSeparator):]
			err = yaml.Unmarshal(frontmatterBytes, &frontmatterObj)
			fmt.Println("Yaml", frontmatterObj)
			if err != nil {
				log.Fatal(err)
			}
		}
		bytesBuffer := bytes.Buffer{}
		err := goldmark.Convert(contentBytes, &bytesBuffer)
		if err != nil {
			log.Fatal(err)
		}
		post := models.Post{
			Frontmatter: frontmatterObj,
			Content:     template.HTML(bytesBuffer.String()),
		}
		fmt.Println("Post", post.Frontmatter.Type)
		if post.Frontmatter.Type == "til" {
			fmt.Println("TTTTTT", post.Frontmatter)
		}
		posts = append(posts, post)
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
		plugins.CleanPostFrontmatter(&post)
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
		fmt.Println(postType, post.Frontmatter.Title)
		if postType == "" {
			postType = "post"
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
		fmt.Println("postSlug", postSlug)
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
			date1, _ := time.Parse("2006-01-02", ssg.Posts[i].Frontmatter.Date)
			date2, _ := time.Parse("2006-01-02", ssg.Posts[j].Frontmatter.Date)
			return date2.Before(date1)
		})
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
		feed := models.Feed{
			Title: postType,
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
