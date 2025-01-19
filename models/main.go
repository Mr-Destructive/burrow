package models

import (
	"html/template"
	"io/fs"
)

const SSG_CONFIG_FILE_NAME string = "ssg.json"

type Author struct {
	Name   string `json:"name"`
	Email  string `json:"email"`
	Github string `json:"github"`
}

type PageConfig struct {
	TemplatePath     string `json:"template"`
	FeedTemplatePath string `json:"feed_template"`
}

type Theme struct {
	Bg            string `json:"bg"`
	Text          string `json:"text"`
	SecondaryText string `json:"secondary-text"`
	Link          struct {
		Normal string `json:"normal"`
		Hover  string `json:"hover"`
		Active string `json:"active"`
	} `json:"link"`
	Quotes     string `json:"quotes"`
	CodeBlocks struct {
		Bg     string `json:"bg"`
		Border string `json:"border"`
	} `json:"codeblocks"`
	Code struct {
		Text     string `json:"text"`
		Comment  string `json:"comment"`
		Keyword  string `json:"keyword"`
		String   string `json:"string"`
		Number   string `json:"number"`
		Variable string `json:"variable"`
		Function string `json:"function"`
	} `json:"code"`
}

type BlogConfig struct {
	Name                string                `json:"name"`
	BaseUrl             string                `json:"base_url"`
	PostsDir            string                `json:"posts_dir"`
	TemplatesDir        string                `json:"templates_dir"`
	StaticDir           string                `json:"static_dir"`
	OutputDir           string                `json:"output_dir"`
	DefaultFeedTemplate string                `json:"default_feed_template"`
	DefaultPostTemplate string                `json:"default_post_template"`
	PrefixURL           string                `json:"prefix_url"`
	PagesConfig         map[string]PageConfig `json:"pages"`
	Themes              map[string]Theme      `json:"themes"`
}

type SSG_CONFIG struct {
	Blog    BlogConfig `json:"blog"`
	Authors []Author   `json:"authors"`
	Plugins []string   `json:"plugins"`
}

var config *SSG_CONFIG

type FrontMatter struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	Date        string `json:"date"`
	Slug        string `json:"slug"`
}

type PostType int

const (
	POST PostType = iota
	TIL
	PROJECT
)

var PostTypes = map[PostType]string{
	POST:    "post",
	TIL:     "til",
	PROJECT: "project",
}

type Post struct {
	Frontmatter FrontMatter
	Content     template.HTML
}

type Feed struct {
	Title string
	Type  string
	Posts []Post
}

type SSG struct {
	Config     SSG_CONFIG
	Posts      []Post
	FeedPosts  []Feed
	TemplateFS *template.Template
	FS         fs.FS
}

type ThemeCombo struct {
	Default   Theme
	Secondary Theme
}
type TemplateContext struct {
	Themes    ThemeCombo
	Config    SSG_CONFIG
	Post      Post
	FeedPosts []Feed
	FeedInfo  Feed
}
