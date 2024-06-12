package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

const peHeaderOffset = 0x3C
const fileHeaderOffset = 0x6

func main() {
	// 定义命令行参数
	filename := flag.String("f", "", "需要更新时间的文件或目录")
	timeStr := flag.String("t", "", "要设置的目标时间（格式：'2006-01-02 15:04:05'）")
	compileTime := flag.Bool("pe", false, "是否更新编译时间")
	randomMinutes := flag.Int("r", 0, "随机时间范围（分钟）")
	flag.Parse()

	if *filename == "" || *timeStr == "" {
		fmt.Println("请使用 -f 和 -t 标志提供文件名或目录名以及目标时间")
		os.Exit(1)
	}

	// 解析目标时间
	targetTime, err := time.Parse("2006-01-02 15:04:05", *timeStr)
	if err != nil {
		fmt.Println("解析时间时出错：", err)
		os.Exit(1)
	}

	info, err := os.Stat(*filename)
	if err != nil {
		fmt.Println("获取文件信息时出错：", err)
		os.Exit(1)
	}

	// 先更新目录本身的时间
	if info.IsDir() {
		if err := updateFileTimes(*filename, targetTime, *compileTime, *randomMinutes); err != nil {
			fmt.Printf("更新目录 %s 时间时出错：%v\n", *filename, err)
			os.Exit(1)
		} else {
			fmt.Printf("成功更新目录 %s 的时间\n", *filename)
		}
	}

	// 遍历目录中的文件和子目录
	err = filepath.Walk(*filename, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if err := updateFileTimes(path, targetTime, *compileTime, *randomMinutes); err != nil {
			fmt.Printf("更新 %s 时间时出错：%v\n", path, err)
		} else {
			fmt.Printf("成功更新 %s 的时间\n", path)
		}
		return nil
	})

	if err != nil {
		fmt.Println("遍历路径时出错：", err)
		os.Exit(1)
	}
}

// 更新文件或目录的时间
func updateFileTimes(filename string, targetTime time.Time, modifyCompileTime bool, randomMinutes int) error {
	if randomMinutes > 0 {
		// 添加随机时间
		offset := time.Duration(rand.Intn(randomMinutes*60)) * time.Second
		targetTime = targetTime.Add(offset)
	}

	// 随机生成访问时间、修改时间、创建时间之间的差距
	accessOffset := time.Duration(rand.Intn(60)) * time.Second
	modifyOffset := accessOffset + time.Duration(rand.Intn(60))*time.Second
	createOffset := modifyOffset + time.Duration(rand.Intn(60))*time.Second

	// 确保访问时间 <= 修改时间 <= 创建时间
	accessTime := targetTime.Add(-accessOffset)
	modifyTime := targetTime.Add(-modifyOffset)
	createTime := targetTime.Add(-createOffset)

	if modifyCompileTime {
		if err := modifyPEFileCompileTime(filename, createTime); err != nil {
			return fmt.Errorf("修改 PE 文件编译时间时出错：%w", err)
		}
	}

	// 更新最后访问时间、修改时间和创建时间
	if err := setFileTimes(filename, createTime, accessTime, modifyTime); err != nil {
		return err
	}

	return nil
}

// 修改 PE 文件的编译时间
func modifyPEFileCompileTime(filename string, targetTime time.Time) error {
	file, err := os.OpenFile(filename, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取 PE 头偏移量
	var offset int32
	if _, err := file.Seek(peHeaderOffset, 0); err != nil {
		return err
	}
	if err := binary.Read(file, binary.LittleEndian, &offset); err != nil {
		return err
	}

	// 读取时间戳
	timeDateStampOffset := int64(offset) + fileHeaderOffset
	if _, err := file.Seek(timeDateStampOffset, 0); err != nil {
		return err
	}

	timestamp := uint32(targetTime.Unix())
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, timestamp); err != nil {
		return err
	}

	if _, err := file.Write(buf.Bytes()); err != nil {
		return err
	}

	return nil
}

// 设置文件或目录的时间
func setFileTimes(filename string, creationTime, lastAccessTime, lastWriteTime time.Time) error {
	path, err := windows.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}
	hFile, err := windows.CreateFile(
		path,
		windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_WRITE|windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(hFile)

	creationTimeFt := windows.NsecToFiletime(creationTime.UnixNano())
	lastAccessTimeFt := windows.NsecToFiletime(lastAccessTime.UnixNano())
	lastWriteTimeFt := windows.NsecToFiletime(lastWriteTime.UnixNano())

	if err := windows.SetFileTime(hFile, &creationTimeFt, &lastAccessTimeFt, &lastWriteTimeFt); err != nil {
		return err
	}
	return nil
}

