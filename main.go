package main

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

func main() {
	// 设置文件服务器的根目录
	rootDir := "./files"

	// 检查根目录是否存在
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		fmt.Println("根目录不存在，创建根目录:", rootDir)
		if err := os.Mkdir(rootDir, 0755); err != nil {
			fmt.Println("创建目录失败:", err)
			return
		}
	}

	http.HandleFunc("/", rootHandler(rootDir))
	http.HandleFunc("/files", fileListHandler(rootDir))
	http.HandleFunc("/upload", uploadHandler(rootDir))

	// 启动服务器
	port := ":8080"
	fmt.Println("文件服务器已启动，监听端口", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		fmt.Println("服务器启动失败:", err)
	}
}

// rootHandler 处理根目录请求
func rootHandler(rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			http.Redirect(w, r, "/files", http.StatusFound)
		} else {
			downloadHandler(w, r, rootDir)
		}
	}
}

// fileListHandler 处理文件列表请求
func fileListHandler(rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(rootDir)
		if err != nil {
			http.Error(w, "无法读取文件列表", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, "<h1>文件列表</h1>")
		fmt.Fprintln(w, "<ul>")
		for _, file := range files {
			if !file.IsDir() {
				filePath := file.Name()
				fmt.Fprintf(w, "<li><a href=\"/%s\">%s</a></li>\n", filePath, filePath)
			}
		}
		fmt.Fprintln(w, "</ul>")
		fmt.Fprintln(w,
			`<form action="/upload" method="post" enctype="multipart/form-data">
		<input type="file" name="file" multiple>
		<input type="submit" value="上传文件">
		</form>`,
		)
	}
}

// downloadHandler 处理文件下载请求
func downloadHandler(w http.ResponseWriter, r *http.Request, rootDir string) {
	filePath := filepath.Join(rootDir, r.URL.Path[1:])
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "文件不存在", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	http.ServeFile(w, r, filePath)
}

// uploadHandler 处理文件上传请求
func uploadHandler(rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "无效的请求方法", http.StatusMethodNotAllowed)
			return
		}

		r.ParseMultipartForm(32 << 20) // 限制上传文件大小为32MB
		var wg sync.WaitGroup
		for _, files := range r.MultipartForm.File {
			for _, handler := range files {
				wg.Add(1)
				go func(handler *multipart.FileHeader) {
					defer wg.Done()

					file, err := handler.Open()
					if err != nil {
						http.Error(w, "无法读取上传的文件", http.StatusBadRequest)
						return
					}
					defer file.Close()

					if handler.Filename == "" {
						http.Error(w, "未选择文件", http.StatusBadRequest)
						return
					}

					filePath := filepath.Join(rootDir, handler.Filename)
					outFile, err := os.Create(filePath)
					if err != nil {
						http.Error(w, "无法创建文件", http.StatusInternalServerError)
						return
					}
					defer outFile.Close()

					if _, err := io.Copy(outFile, file); err != nil {
						http.Error(w, "无法保存文件", http.StatusInternalServerError)
						return
					}
				}(handler)
			}
		}
		wg.Wait()
		http.Redirect(w, r, "/files", http.StatusFound)
	}
}
