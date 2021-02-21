package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
)

const ENCRYPTION_KEY = ""
const HOST_URL = "http://localhost:8080/"
const IMGBB_API_KEY = ""

func encode_bytes_to_image(data []byte) []byte {
	length := len(data)
	quadraticimg_edgelength := int(math.Ceil(math.Sqrt(math.Ceil(float64(length / 3)))))
	img := image.NewRGBA(image.Rect(0, 0, quadraticimg_edgelength, quadraticimg_edgelength))
	current_index := 0
	for y := 0; y < quadraticimg_edgelength; y++ {
		for x := 0; x < quadraticimg_edgelength; x++ {
			var R uint8 = 0
			var G uint8 = 0
			var B uint8 = 0
			if current_index < length {
				R = uint8(data[current_index])
				current_index += 1
			}
			if current_index < length {
				G = uint8(data[current_index])
				current_index += 1
			}
			if current_index < length {
				B = uint8(data[current_index])
				current_index += 1
			}
			img.Set(x, y, color.RGBA{R, G, B, 255})
		}
	}
	img_buffer := new(bytes.Buffer)
	png.Encode(img_buffer, img)
	return img_buffer.Bytes()
}

func decode_image_to_bytes(data []byte, length int64) []byte {
	img, _, _ := image.Decode(bytes.NewReader(data))
	var current_index int64 = 0
	filedata := make([]byte, length)
	bounds := img.Bounds()
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			pixel_color := img.At(x, y)
			R, G, B, _ := pixel_color.RGBA()
			if current_index == length {
				break
			}
			filedata[current_index] = byte(R)
			current_index += 1
			if current_index == length {
				break
			}
			filedata[current_index] = byte(G)
			current_index += 1
			if current_index == length {
				break
			}
			filedata[current_index] = byte(B)
			current_index += 1
		}
	}
	return filedata
}

func bb_image_upload(img_data []byte) []byte {
	form_data := url.Values{
		"expiration": {"15552000"},
		"key":        {IMGBB_API_KEY},
		"image":      {base64.StdEncoding.EncodeToString(img_data)},
	}

	response, _ := http.PostForm("https://api.imgbb.com/1/upload", form_data)
	defer response.Body.Close()
	response_text, _ := ioutil.ReadAll(response.Body)
	return response_text
}

func get_file_by_url(url string, file_size int64) []byte {
	response, _ := http.Get(url)
	defer response.Body.Close()
	response_data, _ := ioutil.ReadAll(response.Body)
	return decode_image_to_bytes(response_data, file_size)
}

func sha256hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func main_handler(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		upload_page_html, _ := ioutil.ReadFile("templates/upload.html")
		w.Header().Add("Strict-Transport-Security", "max-age=17280000;includeSubDomains")
		fmt.Fprintf(w, string(upload_page_html))
		return
	}
	s := strings.Split(req.URL.Path, "/")
	filename, filesize, _, encrypted_size, img_url, _ := get_entry(s[1])
	s = strings.Split(filename, ".")

	if img_url == "" {
		fmt.Fprintf(w, "Error: File not found!")
		return
	}

	filedata := get_file_by_url(img_url, encrypted_size)
	filedata = decryptAES(key, filedata)
	filedata = decompress(filedata)

	if len(s) == 0 || len(s) == 1 || !is_preview_extension(s[1]) {
		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		w.Header().Set("Content-Type", http.DetectContentType(filedata))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", filesize))
		w.Header().Set("x-filename", filename)
		w.Header().Set("Access-Control-Expose-Headers", "x-filename")
	}

	w.Write(filedata)
}

func is_preview_extension(extension string) bool {
	valid_extensions := []string{"pdf", "txt", "py", "c", "go", "sol", "rs", "pb", "md", "js", "css", "html", "png", "gif", "jpg", "jpeg", "mp4"}
	for _, test_extension := range valid_extensions {
		if test_extension == extension {
			return true
		}
	}
	return false
}

func filelist(w http.ResponseWriter, req *http.Request) {
	upload_page_html, _ := ioutil.ReadFile("file.list")
	fmt.Fprintf(w, string(upload_page_html))
	return
}

func datasize_readable(datasize int64) string {
	data_units := [9]string{"B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"}
	du_index := 0
	for datasize >= 1024 {
		du_index += 1
		datasize /= 1024
	}
	return fmt.Sprintf("%d%s", datasize, data_units[du_index])
}

func stats(w http.ResponseWriter, req *http.Request) {
	file, _ := os.Open("file.list")
	defer file.Close()

	var num_files int64 = 0
	var acc_size int64 = 0
	var acc_compressed_size int64 = 0
	var acc_encrypted_size int64 = 0

	s := bufio.NewScanner(file)
	for s.Scan() {
		v := strings.Split(s.Text(), "|")
		size, _ := strconv.ParseInt(v[2], 10, 64)
		compressed_size, _ := strconv.ParseInt(v[3], 10, 64)
		encrypted_size, _ := strconv.ParseInt(v[4], 10, 64)

		num_files += 1
		acc_size += size
		acc_compressed_size += compressed_size
		acc_encrypted_size += encrypted_size
	}

	fmt.Fprintf(w, "Files: %d\n", num_files)
	fmt.Fprintf(w, "Data Size: %s\n", datasize_readable(acc_size))
	fmt.Fprintf(w, "Compressed Size: %s\n", datasize_readable(acc_compressed_size))
	fmt.Fprintf(w, "Encrypted Size: %s\n", datasize_readable(acc_encrypted_size))
	return
}

func uploader(w http.ResponseWriter, req *http.Request) {
	req.Body = http.MaxBytesReader(w, req.Body, 32<<20)
	req.ParseMultipartForm((32 << 20) - 1024)
	file, handler, error := req.FormFile("file")
	if error != nil {
		return
	}
	defer file.Close()

	filedata, _ := ioutil.ReadAll(file)
	sha256hash := sha256hex(filedata)[:16]

	filedata = compress(filedata)
	compressed_size := len(filedata)
	filedata = encryptAES(key, filedata)

	_, _, _, _, url, _ := get_entry(sha256hash)
	if url != "" {
		fmt.Fprintf(w, sha256hash)
		return
	}

	response_text := bb_image_upload(encode_bytes_to_image(filedata))

	var r map[string]interface{}
	json.Unmarshal(response_text, &r)
	data, _ := r["data"].(map[string]interface{})

	file_list.Lock()
	f, _ := os.OpenFile("file.list", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 600)
	fw := bufio.NewWriter(f)
	fmt.Fprintf(fw, "%s|%s|%d|%d|%d|%s|%s\n", sha256hash, handler.Filename, handler.Size, compressed_size, len(filedata), data["url"], data["delete_url"])
	fw.Flush()
	f.Close()
	file_list.Unlock()

	fmt.Fprintf(w, HOST_URL+sha256hash+"\n")
	return
}

func get_entry(sha256hash string) (string, int64, int64, int64, string, string) {
	file, _ := os.Open("file.list")
	defer file.Close()

	s := bufio.NewScanner(file)
	for s.Scan() {
		v := strings.Split(s.Text(), "|")
		if v[0] == sha256hash {
			size, _ := strconv.ParseInt(v[2], 10, 64)
			compressed_size, _ := strconv.ParseInt(v[3], 10, 64)
			encrypted_size, _ := strconv.ParseInt(v[4], 10, 64)
			return v[1], size, compressed_size, encrypted_size, v[5], v[6]
		}
	}
	return "", 0, 0, 0, "", ""
}

func compress(data []byte) []byte {
	var cb bytes.Buffer
	gzw := gzip.NewWriter(&cb)
	gzw.Write(data)
	gzw.Close()
	return cb.Bytes()
}

func decompress(compressed_data []byte) []byte {
	cb := bytes.NewBuffer(compressed_data)
	gzr, _ := gzip.NewReader(cb)
	var db bytes.Buffer
	db.ReadFrom(gzr)
	return db.Bytes()
}

func encryptAES(key []byte, data []byte) []byte {
	block, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, aesgcm.NonceSize())
	encrypted_data := aesgcm.Seal(nonce, nonce, data, nil)
	return encrypted_data
}

func decryptAES(key []byte, encrypted_data []byte) []byte {
	block, _ := aes.NewCipher(key)
	aesgcm, _ := cipher.NewGCM(block)
	nonce_size := aesgcm.NonceSize()
	nonce, encrypted_payload := encrypted_data[:nonce_size], encrypted_data[nonce_size:]
	data, _ := aesgcm.Open(nil, nonce, encrypted_payload, nil)
	return data
}

var file_list sync.Mutex
var key []byte

func main() {
	key = []byte(sha256hex([]byte(ENCRYPTION_KEY))[:32])
	http.HandleFunc("/", main_handler)
	http.HandleFunc("/uploader", uploader)
	http.HandleFunc("/filelist", filelist)
	http.HandleFunc("/stats", stats)
	//http.ListenAndServeTLS(":443", "/etc/letsencrypt/live/SITENAME/fullchain.pem", "/etc/letsencrypt/live/SITENAME/privkey.pem", nil)
	http.ListenAndServe(":8080", nil)
}
