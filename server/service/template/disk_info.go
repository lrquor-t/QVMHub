package template

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

	"qvmhub/utils"
)

// fillTemplateInfoSizes fills disk size info for a single template.
func fillTemplateInfoSizes(tpl *TemplateInfo) {
	if tpl == nil || tpl.Path == "" {
		return
	}
	diskInfo, err := loadTemplateDiskInfo(tpl.Path)
	if err != nil {
		return
	}
	tpl.ActualSize = diskInfo.ActualSize
	tpl.VirtualSize = diskInfo.VirtualSize
	if tpl.FileSize <= 0 {
		tpl.FileSize = diskInfo.FileSize
	}
	if tpl.MD5 == "" || tpl.SHA256 == "" || tpl.FileSize <= 0 {
		tpl.HashStatus = "missing"
	} else if tpl.FileSize != diskInfo.FileSize {
		tpl.HashStatus = "size_mismatch"
	} else {
		tpl.HashStatus = "ok"
	}
}

// fillTemplateInfoSizesBatch fills disk size info for multiple templates concurrently.
func fillTemplateInfoSizesBatch(templates []TemplateInfo) {
	if len(templates) == 0 {
		return
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > templateDiskInfoWorkerLimit {
		workerCount = templateDiskInfoWorkerLimit
	}
	if workerCount > len(templates) {
		workerCount = len(templates)
	}
	if workerCount <= 1 {
		for i := range templates {
			fillTemplateInfoSizes(&templates[i])
		}
		return
	}

	indexCh := make(chan int, len(templates))
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer utils.RecoverAndLog("template-disk-info")
			defer wg.Done()
			for idx := range indexCh {
				fillTemplateInfoSizes(&templates[idx])
			}
		}()
	}
	for i := range templates {
		indexCh <- i
	}
	close(indexCh)
	wg.Wait()
}

// parseQemuInfoBytes parses a key from qemu-img info JSON output.
func parseQemuInfoBytes(output, key string) int64 {
	var data map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return 0
	}
	raw, ok := data[key]
	if !ok {
		return 0
	}
	var bytes int64
	if err := json.Unmarshal(raw, &bytes); err != nil {
		return 0
	}
	return bytes
}

func loadTemplateDiskInfo(path string) (templateDiskInfoCacheEntry, error) {
	info, err := templateDiskInfoStat(path)
	if err != nil {
		return templateDiskInfoCacheEntry{}, err
	}
	size := info.Size()
	modTimeUnixNano := info.ModTime().UnixNano()

	templateDiskInfoCache.RLock()
	cached, ok := templateDiskInfoCache.items[path]
	templateDiskInfoCache.RUnlock()
	if ok && cached.FileSize == size && cached.ModTimeUnixNano == modTimeUnixNano {
		return cached, nil
	}

	entry := templateDiskInfoCacheEntry{
		FileSize:        size,
		ModTimeUnixNano: modTimeUnixNano,
	}
	if size > 0 {
		entry.ActualSize = HookFormatBytesPublic(size)
	}
	result := templateDiskInfoCommand(path)
	if result != nil && result.Error == nil {
		if actualSize := parseQemuInfoBytes(result.Stdout, "actual-size"); actualSize > 0 {
			entry.ActualSize = HookFormatBytesPublic(actualSize)
		}
		if virtualSize := parseQemuInfoBytes(result.Stdout, "virtual-size"); virtualSize > 0 {
			entry.VirtualSize = fmt.Sprintf("%.2f GiB", float64(virtualSize)/float64(1<<30))
		}
	}

	templateDiskInfoCache.Lock()
	templateDiskInfoCache.items[path] = entry
	templateDiskInfoCache.Unlock()
	return entry, nil
}
