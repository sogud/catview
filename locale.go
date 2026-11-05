package main

import (
	"github.com/egoist/mygo/ui"
	"strings"
)

// CatView owns this catalog. translate-kit intentionally has no UI strings.
var uiLanguages = []struct{ id, name string }{
	{"en", "English"}, {"zh-CN", "简体中文"}, {"zh-TW", "繁體中文"}, {"ja", "日本語"}, {"ko", "한국어"}, {"fr", "Français"}, {"de", "Deutsch"}, {"es", "Español"}, {"pt-BR", "Português (Brasil)"}, {"it", "Italiano"}, {"ru", "Русский"}, {"ar", "العربية"}, {"hi", "हिन्दी"}, {"id", "Bahasa Indonesia"}, {"vi", "Tiếng Việt"}, {"th", "ไทย"}, {"tr", "Türkçe"}, {"nl", "Nederlands"},
}
var localeKeys = strings.Split("Settings|Appearance|Reading|Language|System|Light|Dark|File icons|Accent color|Font size|Line spacing|Text width|Standard|Wide|Translation|Provider|Target language|Endpoint|Model|API key|Save|Cancel|Translate document|Cancel translation|Original|Translated|Bilingual|File|View|Open file…|Open folder…|Close tab|Close window|Check for updates…|Edit file|Finish editing|Find|Show sidebar", "|")
var localeRows = map[string]string{
	"zh-CN": "设置|外观|阅读|语言|系统|浅色|深色|文件图标|强调色|字号|行距|正文宽度|标准|宽|翻译|服务商|目标语言|服务地址|模型|API 密钥|保存|取消|翻译文档|取消翻译|原文|译文|双语|文件|显示|打开文件…|打开文件夹…|关闭标签页|关闭窗口|检查更新…|编辑文件|结束编辑|查找|显示侧边栏",
	"zh-TW": "設定|外觀|閱讀|語言|系統|淺色|深色|檔案圖像|強調色|字級|行距|內文寬度|標準|寬|翻譯|服務供應商|目標語言|服務網址|模型|API 金鑰|儲存|取消|翻譯文件|取消翻譯|原文|譯文|雙語|檔案|顯示方式|打開檔案…|打開檔案夾…|關閉分頁|關閉視窗|檢查更新…|編輯檔案|結束編輯|尋找|顯示側邊欄",
	"ja":    "設定|外観|閲覧|言語|システム|ライト|ダーク|ファイルアイコン|アクセントカラー|文字サイズ|行間|本文の幅|標準|広い|翻訳|プロバイダ|翻訳先の言語|エンドポイント|モデル|API キー|保存|キャンセル|書類を翻訳|翻訳をキャンセル|原文|訳文|対訳|ファイル|表示|ファイルを開く…|フォルダを開く…|タブを閉じる|ウインドウを閉じる|アップデートを確認…|ファイルを編集|編集を終了|検索|サイドバーを表示",
	"ko":    "설정|모양|읽기|언어|시스템|라이트|다크|파일 아이콘|강조 색상|글자 크기|줄 간격|본문 너비|표준|넓게|번역|제공업체|번역 언어|서비스 주소|모델|API 키|저장|취소|문서 번역|번역 취소|원문|번역문|원문 및 번역문|파일|보기|파일 열기…|폴더 열기…|탭 닫기|윈도우 닫기|업데이트 확인…|파일 편집|편집 종료|찾기|사이드바 보기",
	"fr":    "Réglages|Apparence|Lecture|Langue|Système|Clair|Sombre|Icônes des fichiers|Couleur d’accentuation|Taille du texte|Interligne|Largeur du texte|Standard|Large|Traduction|Fournisseur|Langue cible|Adresse du service|Modèle|Clé API|Enregistrer|Annuler|Traduire le document|Annuler la traduction|Original|Traduction|Bilingue|Fichier|Présentation|Ouvrir un fichier…|Ouvrir un dossier…|Fermer l’onglet|Fermer la fenêtre|Rechercher des mises à jour…|Modifier le fichier|Terminer la modification|Rechercher|Afficher la barre latérale",
	"de":    "Einstellungen|Erscheinungsbild|Lesen|Sprache|System|Hell|Dunkel|Dateisymbole|Akzentfarbe|Schriftgröße|Zeilenabstand|Textbreite|Standard|Breit|Übersetzung|Anbieter|Zielsprache|Dienstadresse|Modell|API-Schlüssel|Sichern|Abbrechen|Dokument übersetzen|Übersetzung abbrechen|Original|Übersetzung|Zweisprachig|Ablage|Darstellung|Datei öffnen…|Ordner öffnen…|Tab schließen|Fenster schließen|Nach Updates suchen…|Datei bearbeiten|Bearbeitung beenden|Suchen|Seitenleiste einblenden",
	"es":    "Ajustes|Aspecto|Lectura|Idioma|Sistema|Claro|Oscuro|Iconos de archivos|Color de realce|Tamaño del texto|Interlineado|Ancho del texto|Estándar|Amplio|Traducción|Proveedor|Idioma de destino|Dirección del servicio|Modelo|Clave API|Guardar|Cancelar|Traducir documento|Cancelar traducción|Original|Traducción|Bilingüe|Archivo|Visualización|Abrir archivo…|Abrir carpeta…|Cerrar pestaña|Cerrar ventana|Buscar actualizaciones…|Editar archivo|Finalizar edición|Buscar|Mostrar barra lateral",
	"pt-BR": "Ajustes|Aparência|Leitura|Idioma|Sistema|Clara|Escura|Ícones de arquivos|Cor de destaque|Tamanho do texto|Espaçamento entre linhas|Largura do texto|Padrão|Ampla|Tradução|Provedor|Idioma de destino|Endereço do serviço|Modelo|Chave de API|Salvar|Cancelar|Traduzir documento|Cancelar tradução|Original|Tradução|Bilíngue|Arquivo|Visualizar|Abrir arquivo…|Abrir pasta…|Fechar aba|Fechar janela|Buscar atualizações…|Editar arquivo|Concluir edição|Buscar|Mostrar barra lateral",
	"it":    "Impostazioni|Aspetto|Lettura|Lingua|Sistema|Chiaro|Scuro|Icone dei file|Colore di accento|Dimensione del testo|Interlinea|Larghezza del testo|Standard|Ampia|Traduzione|Fornitore|Lingua di destinazione|Indirizzo del servizio|Modello|Chiave API|Salva|Annulla|Traduci documento|Annulla traduzione|Originale|Traduzione|Bilingue|File|Vista|Apri file…|Apri cartella…|Chiudi pannello|Chiudi finestra|Verifica aggiornamenti…|Modifica file|Termina modifica|Trova|Mostra barra laterale",
	"ru":    "Настройки|Оформление|Чтение|Язык|Системная|Светлая|Тёмная|Значки файлов|Цвет акцента|Размер шрифта|Межстрочный интервал|Ширина текста|Обычная|Широкая|Перевод|Сервис|Язык перевода|Адрес сервиса|Модель|Ключ API|Сохранить|Отменить|Перевести документ|Отменить перевод|Оригинал|Перевод|Оригинал и перевод|Файл|Вид|Открыть файл…|Открыть папку…|Закрыть вкладку|Закрыть окно|Проверить обновления…|Редактировать файл|Завершить редактирование|Найти|Показать боковую панель",
	"ar":    "الإعدادات|المظهر|القراءة|اللغة|النظام|فاتح|داكن|أيقونات الملفات|لون التمييز|حجم الخط|تباعد الأسطر|عرض النص|قياسي|واسع|الترجمة|مقدم الخدمة|لغة الترجمة|عنوان الخدمة|النموذج|مفتاح API|حفظ|إلغاء|ترجمة المستند|إلغاء الترجمة|الأصل|الترجمة|ثنائي اللغة|ملف|عرض|فتح ملف…|فتح مجلد…|إغلاق علامة التبويب|إغلاق النافذة|التحقق من التحديثات…|تحرير الملف|إنهاء التحرير|بحث|إظهار الشريط الجانبي",
	"hi":    "सेटिंग्ज़|प्रकटन|पढ़ना|भाषा|सिस्टम|हल्का|गहरा|फ़ाइल आइकॉन|ऐक्सेंट रंग|फ़ॉन्ट का आकार|पंक्तियों के बीच दूरी|टेक्स्ट की चौड़ाई|मानक|चौड़ा|अनुवाद|सेवा प्रदाता|अनुवाद की भाषा|सेवा का पता|मॉडल|API कुंजी|सहेजें|रद्द करें|दस्तावेज़ का अनुवाद करें|अनुवाद रद्द करें|मूल|अनुवाद|दोनों भाषाएँ|फ़ाइल|दृश्य|फ़ाइल खोलें…|फ़ोल्डर खोलें…|टैब बंद करें|विंडो बंद करें|अपडेट जाँचें…|फ़ाइल संपादित करें|संपादन समाप्त करें|खोजें|साइडबार दिखाएँ",
	"id":    "Pengaturan|Tampilan|Membaca|Bahasa|Sistem|Terang|Gelap|Ikon file|Warna aksen|Ukuran font|Jarak baris|Lebar teks|Standar|Lebar|Terjemahan|Penyedia|Bahasa tujuan|Alamat layanan|Model|Kunci API|Simpan|Batal|Terjemahkan dokumen|Batalkan terjemahan|Asli|Terjemahan|Dua bahasa|File|Lihat|Buka file…|Buka folder…|Tutup tab|Tutup jendela|Periksa pembaruan…|Edit file|Selesai mengedit|Cari|Tampilkan bar samping",
	"vi":    "Cài đặt|Giao diện|Đọc|Ngôn ngữ|Hệ thống|Sáng|Tối|Biểu tượng tệp|Màu nhấn|Cỡ chữ|Giãn dòng|Độ rộng văn bản|Tiêu chuẩn|Rộng|Dịch|Nhà cung cấp|Ngôn ngữ đích|Địa chỉ dịch vụ|Mô hình|Khóa API|Lưu|Hủy|Dịch tài liệu|Hủy dịch|Bản gốc|Bản dịch|Song ngữ|Tệp|Xem|Mở tệp…|Mở thư mục…|Đóng tab|Đóng cửa sổ|Kiểm tra bản cập nhật…|Sửa tệp|Kết thúc chỉnh sửa|Tìm|Hiển thị thanh bên",
	"th":    "การตั้งค่า|รูปลักษณ์|การอ่าน|ภาษา|ระบบ|สว่าง|มืด|ไอคอนไฟล์|สีเน้น|ขนาดตัวอักษร|ระยะห่างบรรทัด|ความกว้างข้อความ|มาตรฐาน|กว้าง|การแปล|ผู้ให้บริการ|ภาษาปลายทาง|ที่อยู่บริการ|โมเดล|คีย์ API|บันทึก|ยกเลิก|แปลเอกสาร|ยกเลิกการแปล|ต้นฉบับ|คำแปล|สองภาษา|ไฟล์|มุมมอง|เปิดไฟล์…|เปิดโฟลเดอร์…|ปิดแถบ|ปิดหน้าต่าง|ตรวจหารายการอัปเดต…|แก้ไขไฟล์|เสร็จสิ้นการแก้ไข|ค้นหา|แสดงแถบด้านข้าง",
	"tr":    "Ayarlar|Görünüş|Okuma|Dil|Sistem|Açık|Koyu|Dosya simgeleri|Vurgu rengi|Yazı tipi boyutu|Satır aralığı|Metin genişliği|Standart|Geniş|Çeviri|Sağlayıcı|Hedef dil|Hizmet adresi|Model|API anahtarı|Kaydet|İptal|Belgeyi çevir|Çeviriyi iptal et|Özgün|Çeviri|İki dilli|Dosya|Görüntü|Dosya aç…|Klasör aç…|Sekmeyi kapat|Pencereyi kapat|Güncellemeleri denetle…|Dosyayı düzenle|Düzenlemeyi bitir|Bul|Kenar çubuğunu göster",
	"nl":    "Instellingen|Weergave|Lezen|Taal|Systeem|Licht|Donker|Bestandssymbolen|Accentkleur|Tekstgrootte|Regelafstand|Tekstbreedte|Standaard|Breed|Vertaling|Aanbieder|Doeltaal|Serviceadres|Model|API-sleutel|Bewaar|Annuleer|Vertaal document|Annuleer vertaling|Origineel|Vertaling|Tweetalig|Archief|Weergave|Open bestand…|Open map…|Sluit tabblad|Sluit venster|Zoek naar updates…|Bewerk bestand|Stop bewerken|Zoek|Toon navigatiekolom",
}
var localeCatalog = func() map[string]map[string]string {
	result := make(map[string]map[string]string)
	for language, row := range localeRows {
		values := strings.Split(row, "|")
		catalog := make(map[string]string)
		for i, key := range localeKeys {
			if i < len(values) {
				catalog[key] = values[i]
			}
		}
		result[language] = catalog
	}
	return result
}()

func matchLanguage(tag string) string {
	tag = strings.ToLower(strings.ReplaceAll(tag, "_", "-"))
	tag = strings.Split(strings.Split(tag, ".")[0], "@")[0]
	parts := strings.Split(tag, "-")
	if parts[0] == "zh" {
		for _, part := range parts {
			if part == "hant" {
				return "zh-TW"
			}
		}
		for _, part := range parts {
			if part == "hans" {
				return "zh-CN"
			}
		}
		for _, part := range parts {
			if part == "tw" || part == "hk" || part == "mo" {
				return "zh-TW"
			}
		}
		return "zh-CN"
	}
	if parts[0] == "pt" {
		return "pt-BR"
	}
	for _, language := range uiLanguages {
		if strings.ToLower(language.id) == parts[0] {
			return language.id
		}
	}
	return ""
}
func (a *application) resolveLanguage() {
	a.resolvedLanguage = matchLanguage(a.language)
	if a.language == "" || a.language == "system" {
		a.resolvedLanguage = systemLanguage()
	}
	if a.resolvedLanguage == "" {
		a.resolvedLanguage = "en"
	}
}
func (a *application) tr(key string) string {
	if value := localeCatalog[a.resolvedLanguage][key]; value != "" {
		return value
	}
	return key // English source text is the fallback.
}
func (a *application) languageSettings(c *ui.Context) {
	labels := []string{a.tr("System")}
	selected := labels[0]
	for _, language := range uiLanguages {
		labels = append(labels, language.name)
		if a.language == language.id {
			selected = language.name
		}
	}
	ui.Field(c, a.tr("Language"), func() {
		if ui.Select(c, &selected, labels).Width(240).AlignSelf(ui.Start).Label(a.tr("Language")).Changed() {
			a.language = "system"
			for _, language := range uiLanguages {
				if language.name == selected {
					a.language = language.id
					break
				}
			}
			a.resolveLanguage()
			a.storePreferences()
			a.installMenu()
			a.win.Invalidate()
			if a.settings != nil {
				a.settings.SetTitle(a.tr("Settings"))
				a.settings.Invalidate()
			}
		}
	})
}

func init() {
	extra := map[string]string{
		"Close other tabs": "关闭其他标签", "Close tabs to the left": "关闭左侧标签", "Close tabs to the right": "关闭右侧标签", "Close all tabs": "关闭全部标签", "Copy path": "复制路径",
		"New window": "新建窗口", "Open folder in new window…": "在新窗口打开文件夹…", "Recent items": "最近打开", "No recent items": "暂无打开记录", "Open history…": "打开历史…", "Open history": "打开历史", "Clear history": "清空历史", "Open in a new window": "在新窗口打开", "Folder": "文件夹", "Could not save history: ": "无法保存历史：", "This history item is no longer available.": "无法打开：历史记录中的文件或文件夹已不可用。",
		"System preview":      "系统预览",
		"Image page %d of %d": "第 %d 页，共 %d 页", "Previous image page": "上一张图片", "Next image page": "下一张图片",
		"Opening preview…": "正在加载预览…", "Try again": "重试", "Open in default app": "用默认应用打开",
		"Reading font": "阅读字体", "Code font": "代码字体",
		"Reset text size / fit image":  "恢复字号／图片适应窗口",
		"Zoom in / increase text size": "放大图片／文字", "Zoom out / decrease text size": "缩小图片／文字",
		"Fit image to window": "图片适应窗口", "Image at 100%": "图片缩放至 100%", "Show source": "显示源码",
		"Copy cell": "复制单元格", "Empty table": "表格为空",
		"Preview limited to 20,000 rows, 128 columns or 200,000 cells.":                                    "仅预览前 20,000 行、128 列，最多 200,000 个单元格。",
		"Archive contents only. Files are not extracted.":                                                  "仅浏览压缩包目录，不解压文件。",
		"This media could not be opened. Its codec may not be supported by macOS, or the file is damaged.": "无法打开此媒体。macOS 可能不支持其编码，或文件已损坏。",
		"Recent folders": "最近打开的文件夹", "No recent folders": "无最近文件夹", "Clear recent folders": "清除最近记录",
		"Show in Finder": "在访达中显示", "Refresh folders": "重新读取目录", "Increase text size": "放大文字", "Decrease text size": "缩小文字",
		"Choose a file": "选择文件", "Choose a folder": "选择文件夹", "⌘O Open a file": "⌘O 打开文件", "Toggle sidebar ⌘B": "显示或隐藏侧边栏 ⌘B", "Open folder ⇧⌘O": "打开文件夹 ⇧⌘O",
		"Preview unavailable": "暂不支持预览", "Open with another app in Finder.": "可以通过访达使用其他应用打开。", "Opening PDF…": "正在打开 PDF…", "Using the system PDF viewer.": "使用系统 PDF 阅读器。", "EPUB preview unavailable": "EPUB 无法预览", "Try another EPUB file.": "请尝试其他 EPUB 文件。",
		"Translating…": "正在翻译…", "Could not save settings": "无法保存设置", "Saved locally": "已保存到本地",
		"Only explicit translation sends text to the selected provider. Google and Youdao web services are unofficial.": "仅主动翻译时向所选服务发送正文。Google 和有道网页接口非官方。",
		"Keys are saved locally as plain text with user-only file permissions, not in Keychain.":                        "密钥以明文保存在本地，仅当前用户可读写，不使用钥匙串。",
		"Send this document to the selected translation provider?":                                                      "将当前文档发送给以下翻译服务？", "The original file will not change.": "不会修改原文件。",
		"Translate a document smaller than 1 MiB":                   "请选择小于 1 MiB 的文档",
		"Translation failed. Check provider settings or try again.": "翻译失败。请检查服务设置后重试。",
		"Graphite": "石墨", "Blue": "雾蓝", "Green": "苔绿",
	}
	for key, value := range extra {
		localeCatalog["zh-CN"][key] = value
	}
}

func (a *application) translatorForUI() func(string) string {
	// Dialogs may run in a goroutine: snapshot the selected locale first.
	language := a.resolvedLanguage
	return func(key string) string {
		if value := localeCatalog[language][key]; value != "" {
			return value
		}
		return key
	}
}
func init() {
	simplified := map[string]string{
		"You’re up to date": "当前已是最新版本", "OK": "好", "Could not check for updates": "无法检查更新", "Please try again later.": "请稍后重试。",
		"CatView %s is available": "有新版本 CatView %s", "Current version: %s. Download from the official release page; installation is manual.": "当前版本：%s。前往官方发布页手动下载，不会自动安装。",
		"Open download page": "打开下载页", "Later": "稍后", "Could not open download page: ": "无法打开下载页：",
		"Could not connect to GitHub": "连接 GitHub 失败", "Could not read GitHub release information": "无法读取 GitHub 版本信息", "No public release is available": "尚无可用的公开版本",
		"Save changes to %s?": "保存对 %s 的修改？", "Don’t Save": "不保存", "Cannot close while saving. Please try again.": "无法关闭：文件正在保存，请稍后重试。",
		"Cannot close: new edits appeared while saving. Save again.": "无法关闭：保存期间出现新改动，请再次保存。", "Cannot close: ": "无法关闭：",
		"Discard unsaved changes?": "放弃未保存的改动？", "These changes have not been saved to disk.": "这些改动尚未写入磁盘。", "Keep Editing": "继续编辑", "Discard Changes": "放弃改动",
		"Cannot finish editing while saving. Please try again.": "无法结束编辑：文件正在保存，请稍后重试。", "Changes discarded": "已放弃改动", "Reading mode": "阅读模式", "Saving…": "正在保存…", "Could not save: ": "保存失败：", "Saved ": "已保存 ",
		"The target was moved, replaced, or is not a regular file": "目标已被移动、替换或不是普通文件", "The file changed outside CatView. Reopen it or save a copy.": "文件已被外部程序修改；请重新打开或另存。",
	}
	traditional := map[string]string{
		"You’re up to date": "目前已是最新版本", "OK": "好", "Could not check for updates": "無法檢查更新", "Please try again later.": "請稍後再試。",
		"CatView %s is available": "有新版本 CatView %s", "Current version: %s. Download from the official release page; installation is manual.": "目前版本：%s。前往官方發佈頁手動下載，不會自動安裝。",
		"Open download page": "打開下載頁", "Later": "稍後", "Could not open download page: ": "無法打開下載頁：",
		"Could not connect to GitHub": "連線 GitHub 失敗", "Could not read GitHub release information": "無法讀取 GitHub 版本資訊", "No public release is available": "尚無可用的公開版本",
		"Save changes to %s?": "儲存對 %s 的修改？", "Don’t Save": "不儲存", "Cannot close while saving. Please try again.": "無法關閉：檔案正在儲存，請稍後再試。",
		"Cannot close: new edits appeared while saving. Save again.": "無法關閉：儲存期間出現新修改，請再次儲存。", "Cannot close: ": "無法關閉：",
		"Discard unsaved changes?": "放棄未儲存的修改？", "These changes have not been saved to disk.": "這些修改尚未寫入磁碟。", "Keep Editing": "繼續編輯", "Discard Changes": "放棄修改",
		"Cannot finish editing while saving. Please try again.": "無法結束編輯：檔案正在儲存，請稍後再試。", "Changes discarded": "已放棄修改", "Reading mode": "閱讀模式", "Saving…": "正在儲存…", "Could not save: ": "儲存失敗：", "Saved ": "已儲存 ",
		"The target was moved, replaced, or is not a regular file": "目標已被移動、替換或不是一般檔案", "The file changed outside CatView. Reopen it or save a copy.": "檔案已被外部程式修改；請重新打開或另存。",
	}
	for key, value := range simplified {
		localeCatalog["zh-CN"][key] = value
	}
	for key, value := range traditional {
		localeCatalog["zh-TW"][key] = value
	}
}

func init() {
	entries := map[string][2]string{
		"The downloaded update is missing or damaged. Download it again.": {"已下载的更新丢失或损坏，请重新下载。", "已下載的更新遺失或損壞，請重新下載。"},
		"Updates": {"更新", "更新"}, "Software update": {"软件更新", "軟體更新"},
		"Checking for updates…":   {"正在检查更新…", "正在檢查更新…"},
		"Downloading update…":     {"正在下载更新…", "正在下載更新…"},
		"Installing update…":      {"正在安装更新…", "正在安裝更新…"},
		"Update ready to install": {"更新已就绪", "更新已就緒"},
		"An update is available":  {"发现新版本", "有新版本可用"},
		"Install and Restart":     {"安装并重启", "安裝並重新啟動"},
		"Download update":         {"下载更新", "下載更新"}, "View update": {"查看更新", "查看更新"}, "Retry": {"重试", "重試"},
		"Automatically check for updates":                                   {"自动检查更新", "自動檢查更新"},
		"Automatically download updates":                                    {"自动下载更新", "自動下載更新"},
		"Checks every six hours. Installation always waits for your click.": {"每 6 小时检查一次，下载完成后由你点击安装并重启。", "每 6 小時檢查一次，下載完成後由你點擊安裝並重新啟動。"},
		"Last checked": {"上次检查", "上次檢查"},
		"The update is downloaded and verified. Install when you are ready.":             {"更新已下载并通过签名验证，可在方便时安装并重启。", "更新已下載並通過簽章驗證，可在方便時安裝並重新啟動。"},
		"Download paused. You can try again later.":                                      {"下载已取消，可稍后重新下载。", "下載已取消，可稍後重新下載。"},
		"Finish the current file operation before installing.":                           {"请先完成当前文件操作，再安装更新。", "請先完成目前的檔案操作，再安裝更新。"},
		"Save or discard unsaved changes before installing the update.":                  {"请先保存或放弃未保存的编辑，再安装更新。", "請先儲存或放棄尚未儲存的編輯，再安裝更新。"},
		"Move CatView to a writable Applications folder to install updates.":             {"请将 CatView 移到有写入权限的“应用程序”文件夹，再安装更新。", "請將 CatView 移至可寫入的「應用程式」資料夾，再安裝更新。"},
		"GitHub is temporarily limiting requests. Try again after %s.":                   {"GitHub 暂时限制了请求，请在 %s 后重试。", "GitHub 暫時限制了請求，請在 %s 後重試。"},
		"GitHub is refusing requests (HTTP %d). Retry later or open the download page.":  {"GitHub 拒绝了请求（HTTP %d），可稍后重试或打开下载页。", "GitHub 拒絕了請求（HTTP %d），可稍後重試或開啟下載頁。"},
		"Could not connect to GitHub. Check your connection or retry later.":             {"无法连接 GitHub，请检查网络或稍后重试。", "無法連線 GitHub，請檢查網路或稍後重試。"},
		"This release requires a manual download. Open the download page to install it.": {"此版本需要手动下载，请打开下载页安装。", "此版本需要手動下載，請開啟下載頁安裝。"},
		"Incomplete update download":                                                     {"更新未完整下载，请重试。", "更新未完整下載，請重試。"},
		"Update signature verification failed. The app was not changed.":                 {"更新签名验证失败，应用未被修改。", "更新簽章驗證失敗，應用程式未被修改。"},
		"Invalid update manifest":                                                        {"更新信息无效，请稍后重试。", "更新資訊無效，請稍後重試。"},
		"Invalid update download URL":                                                    {"更新下载地址无效。", "更新下載網址無效。"},
		"Invalid update signature":                                                       {"更新签名无效。", "更新簽章無效。"},
		"Invalid update public key":                                                      {"更新公钥无效。", "更新公鑰無效。"},
		"Invalid CatView update bundle":                                                  {"CatView 更新包无效。", "CatView 更新套件無效。"},
		"Update version does not match the release":                                      {"更新包与发布版本不匹配。", "更新套件與發佈版本不符。"},
		"Could not create the update cache":                                              {"无法创建更新缓存。", "無法建立更新快取。"},
		"Open":                                                                           {"打开", "打開"},
		"Open with default app":                                                          {"用默认应用打开", "以預設應用程式打開"},
		"New file…":                                                                      {"新建文件…", "新增檔案…"},
		"New folder…":                                                                    {"新建文件夹…", "新增資料夾…"},
		"Rename…":                                                                        {"重命名…", "重新命名…"},
		"Move to Trash…":                                                                 {"移到废纸篓…", "移到垃圾桶…"},
		"Move to Trash":                                                                  {"移到废纸篓", "移到垃圾桶"},
		"Name":                                                                           {"名称", "名稱"}, "Confirm": {"确认", "確認"},
		"Untitled.txt": {"未命名.txt", "未命名.txt"}, "Untitled folder": {"未命名文件夹", "未命名資料夾"},
		"Move “%s” to Trash?":                                                       {"将“%s”移到废纸篓？", "將「%s」移到垃圾桶？"},
		"The item and its contents can be restored from the Trash.":                 {"项目及其中的内容可从废纸篓恢复。", "項目及其中的內容可從垃圾桶復原。"},
		"Save or discard edits in this item before renaming or moving it to Trash.": {"请先保存或放弃此项目中的未保存编辑，再重命名或移到废纸篓。", "請先儲存或放棄此項目中的編輯，再重新命名或移到垃圾桶。"},
		"Cannot change files: ":                                                     {"无法修改文件：", "無法修改檔案："},
		"That name already exists. Choose another name.":                            {"名称已存在，请使用其他名称。", "名稱已存在，請使用其他名稱。"},
		"Permission denied. Check the folder’s permissions in Finder.":              {"没有操作权限，请在访达中检查文件夹权限。", "沒有操作權限，請在 Finder 中檢查資料夾權限。"},
		"Diagram could not be rendered. The source is shown below.":                 {"图表暂时无法渲染，以下保留原始代码。", "圖表暫時無法繪製，以下保留原始程式碼。"},
		"Cannot open link":                                         {"无法打开链接", "無法打開連結"},
		"The linked file could not be found.":                      {"找不到链接指向的文件。", "找不到連結指向的檔案。"},
		"The linked file is outside the opened folder.":            {"链接文件不在已打开的文件夹内，请先打开其所在文件夹。", "連結檔案不在已開啟的資料夾內，請先開啟其所在資料夾。"},
		"This link does not point to a file.":                      {"此链接没有指向文件。", "此連結沒有指向檔案。"},
		"The item no longer exists. Refresh the folder.":           {"项目已不存在，请刷新文件夹。", "項目已不存在，請重新整理資料夾。"},
		"Enter a name without slashes, colons, or line breaks.":    {"请输入不含斜杠、冒号或换行的名称。", "請輸入不含斜線、冒號或換行的名稱。"},
		"The parent is no longer a folder.":                        {"上级路径已不再是文件夹。", "上層路徑已不再是資料夾。"},
		"The item changed outside CatView. Refresh and try again.": {"项目已被其他程序替换，请刷新后再试。", "項目已被其他程式取代，請重新整理後再試。"},
		"Color theme": {"配色主题", "配色主題"}, "Paper": {"暖白", "暖白"},
		"The selected folder changed. Close this dialog and try again.": {"当前文件夹已切换，请关闭对话框后重试。", "目前資料夾已切換，請關閉對話框後重試。"},
		"Forest": {"森林", "森林"}, "Lavender": {"薰衣草", "薰衣草"}, "Sand": {"砂岩", "砂岩"},
		"Purple": {"柔紫", "柔紫"}, "Amber": {"琥珀", "琥珀"}, "Teal": {"青绿", "青綠"},
		"Rose": {"玫瑰", "玫瑰"}, "Orange": {"暖橘", "暖橘"},
		"Theme preview": {"主题预览", "主題預覽"}, "Reading preview": {"中文与 English，安静地阅读。", "中文與 English，安靜地閱讀。"},
		"Open and read.": {"打开，即读。", "打開，即讀。"}, "Link": {"链接", "連結"},
	}
	for key, pair := range entries {
		localeCatalog["zh-CN"][key] = pair[0]
		localeCatalog["zh-TW"][key] = pair[1]
	}
}
