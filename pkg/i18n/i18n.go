package i18n

import "sync"

// UserLanguageStore manages in-memory language preferences for users.
type UserLanguageStore struct {
	mu        sync.RWMutex
	languages map[int64]string
}

// GlobalUserLangStore is the global instance for fast language lookup.
var GlobalUserLangStore = &UserLanguageStore{
	languages: make(map[int64]string),
}

func (s *UserLanguageStore) Get(userID int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if lang, exists := s.languages[userID]; exists && lang != "" {
		return lang
	}
	return "en"
}

func (s *UserLanguageStore) Set(userID int64, lang string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := Translations[lang]; exists {
		s.languages[userID] = lang
	} else {
		s.languages[userID] = "en"
	}
}

// TextBundle holds all string templates for a specific language.
type TextBundle struct {
	LanguageName   string
	Flag           string
	StartMessage   string
	HelpMessage    string
	AboutMessage   string
	SelectLang     string
	LangUpdated    string
	ProcessingURL  string
	ResolvingMedia string
	UploadingMedia string
	Done           string
	InvalidURL     string
	RateLimited    string
	DownloadError  string
	UploadFallback string
	ForceSubMsg    string
	BtnJoinChannel string
	BtnJoined      string
	BtnHelp        string
	BtnAbout       string
	BtnLanguage    string
	BtnDirectLink  string
}

// SupportedLanguages lists the code and display info for all 12 requested languages.
var SupportedLanguages = []struct {
	Code string
	Name string
	Flag string
}{
	{"en", "English", "🇺🇸"},
	{"hi", "हिन्दी (Hindi)", "🇮🇳"},
	{"ta", "தமிழ் (Tamil)", "🇮🇳"},
	{"te", "తెలుగు (Telugu)", "🇮🇳"},
	{"ar", "العربية (Arabic)", "🇸🇦"},
	{"ur", "اردو (Urdu)", "🇵🇰"},
	{"bn", "বাংলা (Bangla)", "🇧🇩"},
	{"zh", "中文 (Chinese)", "🇨🇳"},
	{"ru", "Русский (Russian)", "🇷🇺"},
	{"ko", "한국어 (Korean)", "🇰🇷"},
	{"ja", "日本語 (Japanese)", "🇯🇵"},
	{"tr", "Türkçe (Turkish)", "🇹🇷"},
}

// Translations map holding all dynamic text strings across the 12 languages.
var Translations = map[string]TextBundle{
	"en": {
		LanguageName:   "English",
		Flag:           "🇺🇸",
		StartMessage:   "✨ <b>Welcome to Instagram Downloader!</b>\n\n<blockquote>🚀 <i>High-Speed Serverless Media Delivery</i></blockquote>\n\n📥 Send me any Instagram link to instantly download:\n• 🎬 <b>Reels</b>\n• 📸 <b>Posts & Carousels</b>\n• 📹 <b>Videos & Photos</b>\n\n⚡ <i>No login required • Free forever • Original HD quality</i>",
		HelpMessage:    "📖 <b>How to Use:</b>\n\n1️⃣ Open Instagram and copy the share link of any Reel, Video, or Post.\n2️⃣ Paste and send the link to this chat.\n3️⃣ The bot will instantly fetch and deliver the HD media right here.\n\n💡 <i>Supports single videos, photos, and multi-slide albums/carousels.</i>",
		AboutMessage:   "💎 <b>About This Bot:</b>\n\n• ⚡ <b>Engine:</b> Ultra-Fast Serverless Go\n• 🔄 <b>Resolvers:</b> Multi-Source Intelligent Fallback\n• 🛡 <b>Privacy:</b> Zero permanent storage & auto-cleanup\n• 🌐 <b>International:</b> 12 Native Languages Supported",
		SelectLang:     "🌐 <b>Choose your preferred language:</b>\n\n<i>Tap any option below to change your language:</i>",
		LangUpdated:    "✅ Language updated to <b>English</b>!",
		ProcessingURL:  "🔍 <b>Analyzing Instagram link...</b>",
		ResolvingMedia: "⚡ <b>Extracting HD media streams...</b>",
		UploadingMedia: "🚀 <b>Sending media to your chat...</b>",
		Done:           "✨ <b>Downloaded successfully!</b>",
		InvalidURL:     "⚠️ <b>Invalid Link!</b>\n\nPlease send a valid Instagram URL:\n<code>https://www.instagram.com/reel/...</code>",
		RateLimited:    "⏳ <b>Slow down!</b>\nToo many requests. Please wait a few seconds before trying again.",
		DownloadError:  "❌ <b>Could not retrieve media from this link.</b>\n<i>The post might be private, deleted, or age-restricted.</i>",
		UploadFallback: "⚠️ <b>Could not upload file directly to Telegram.</b>\n\n🔗 <b>Direct download link:</b>",
		ForceSubMsg:    "📢 <b>Channel Subscription Required</b>\n\nTo use this bot and download unlimited media, please join our official updates channel first:\n\n👉 Click <b>Join Channel</b> below, then tap <b>Joined ✅</b> to continue!",
		BtnJoinChannel: "📢 Join Channel",
		BtnJoined:      "Joined ✅",
		BtnHelp:        "📖 Help Guide",
		BtnAbout:       "ℹ️ About Bot",
		BtnLanguage:    "🌐 Language",
		BtnDirectLink:  "🔗 Open Direct Link",
	},
	"hi": {
		LanguageName:   "हिन्दी",
		Flag:           "🇮🇳",
		StartMessage:   "👋 <b>इंस्टाग्राम डाउनलोडर बॉट में आपका स्वागत है!</b>\n\nमुझे कोई भी इंस्टाग्राम लिंक (<b>Reel, Post, Video, Photo या Carousel</b>) भेजें और मैं इसे तुरंत डाउनलोड करके भेजूंगा!",
		HelpMessage:    "📖 <b>उपयोग कैसे करें:</b>\n1. किसी भी इंस्टाग्राम रील, पोस्ट या वीडियो का लिंक कॉपी करें।\n2. बॉट को भेजें।\n3. मीडिया कुछ ही पलों में आपको मिल जाएगा।",
		AboutMessage:   "ℹ️ <b>बॉट के बारे में:</b>\n• सर्वरलेस गो इंजन द्वारा संचालित\n• मल्टिपल रिज़ॉल्वर बैकअप\n• हाई क्वालिटी मीडिया डिलीवरी\n• पूर्णतः सुरक्षित",
		SelectLang:     "🌐 <b>अपनी पसंदीदा भाषा चुनें:</b>",
		LangUpdated:    "✅ भाषा बदलकर <b>हिन्दी</b> कर दी गई है!",
		ProcessingURL:  "🔎 <b>इंस्टाग्राम लिंक की जांच हो रही है...</b>",
		ResolvingMedia: "⬇️ <b>मीडिया प्राप्त किया जा रहा है...</b>",
		UploadingMedia: "⬆️ <b>टेलीग्राम पर मीडिया अपलोड हो रहा है...</b>",
		Done:           "✅ <b>सफलतापूर्वक डाउनलोड हो गया!</b>",
		InvalidURL:     "⚠️ कृपया एक मान्य इंस्टाग्राम लिंक भेजें (जैसे https://www.instagram.com/reel/...)।",
		RateLimited:    "⏳ बहुत सारे अनुरोध! कृपया अगला लिंक भेजने से पहले थोड़ा प्रतीक्षा करें।",
		DownloadError:  "❌ इस इंस्टाग्राम लिंक से मीडिया प्राप्त नहीं किया जा सका।",
		UploadFallback: "❌ <b>टेलीग्राम पर सीधे अपलोड नहीं किया जा सका।</b>\n\n🔗 <b>सीधा डाउनलोड लिंक:</b>",
		BtnHelp:        "📖 सहायता",
		BtnAbout:       "ℹ️ जानकारी",
		BtnLanguage:    "🌐 भाषा बदलें",
		BtnDirectLink:  "🔗 डायरेक्ट लिंक",
	},
	"ta": {
		LanguageName:   "தமிழ்",
		Flag:           "🇮🇳",
		StartMessage:   "👋 <b>இன்ஸ்டாகிராம் டவுன்லோடர் போட்க்கு நல்வரவு!</b>\n\nஎந்தவொரு இன்ஸ்டாகிராம் இணைப்பையும் (<b>Reel, Post, Video, Photo</b>) அனுப்பினால், உடனடியாகப் பதிவிறக்கி தருகிறேன்!",
		HelpMessage:    "📖 <b>பயன்படுத்துவது எப்படி:</b>\n1. இன்ஸ்டாகிராம் பதிவின் இணைப்பை நகலெடுக்கவும்.\n2. இந்த போட்க்கு அனுப்பவும்.\n3. மீடியா உடனடியாக பதிவிறக்கப்படும்.",
		AboutMessage:   "ℹ️ <b>போட் பற்றி:</b>\n• அதிவேக சர்வர்லெஸ் கோ எஞ்சின்\n• பல மாற்று வழங்குநர்கள்\n• உயர்தர பதிவிறக்கம்",
		SelectLang:     "🌐 <b>உங்கள் மொழியைத் தேர்ந்தெடுக்கவும்:</b>",
		LangUpdated:    "✅ மொழி <b>தமிழ்</b> என மாற்றப்பட்டது!",
		ProcessingURL:  "🔎 <b>இணைப்பு சரிபார்க்கப்படுகிறது...</b>",
		ResolvingMedia: "⬇️ <b>மீடியா பெறப்படுகிறது...</b>",
		UploadingMedia: "⬆️ <b>டெலிகிராமில் பதிவேற்றப்படுகிறது...</b>",
		Done:           "✅ <b>வெற்றிகரமாக பதிவிறக்கப்பட்டது!</b>",
		InvalidURL:     "⚠️ சரியான இன்ஸ்டாகிராம் இணைப்பை அனுப்பவும்.",
		RateLimited:    "⏳ சற்று காத்திருந்து மீண்டும் முயற்சிக்கவும்.",
		DownloadError:  "❌ இந்த இணைப்பிலிருந்து பதிவிறக்க முடியவில்லை.",
		UploadFallback: "❌ <b>நேரடியாக பதிவேற்ற முடியவில்லை.</b>\n\n🔗 <b>நேரடி பதிவிறக்க இணைப்பு:</b>",
		BtnHelp:        "📖 உதவி",
		BtnAbout:       "ℹ️ விவரம்",
		BtnLanguage:    "🌐 மொழி",
		BtnDirectLink:  "🔗 நேரடி இணைப்பு",
	},
	"te": {
		LanguageName:   "తెలుగు",
		Flag:           "🇮🇳",
		StartMessage:   "👋 <b>ఇన్‌స్టాగ్రామ్ డౌన్‌లోడర్ బాట్‌కు స్వాగతం!</b>\n\nఏదైనా ఇన్‌స్టాగ్రామ్ లింక్ (<b>Reel, Post, Video, Photo</b>) పంపండి, తక్షణమే డౌన్‌లోడ్ చేసి పంపుతాను!",
		HelpMessage:    "📖 <b>ఎలా ఉపయోగించాలి:</b>\n1. ఇన్‌స్టాగ్రామ్ లింక్ కాపీ చేయండి.\n2. ఈ బాట్‌కు పంపండి.\n3. క్షణాల్లో మీడియా అందుతుంది.",
		AboutMessage:   "ℹ️ <b>బాట్ వివరాలు:</b>\n• హై-స్పీడ్ సర్వర్‌లెస్ గో ఇంజిన్\n• అత్యుత్తమ రిజాల్వర్ సపోర్ట్\n• హై క్వాలిటీ డౌన్‌లోడ్",
		SelectLang:     "🌐 <b>మీ భాషను ఎంచుకోండి:</b>",
		LangUpdated:    "✅ భాష <b>తెలుగు</b> గా మార్చబడింది!",
		ProcessingURL:  "🔎 <b>లింక్ ప్రాసెస్ చేయబడుతోంది...</b>",
		ResolvingMedia: "⬇️ <b>మీడియా పొందబడుతోంది...</b>",
		UploadingMedia: "⬆️ <b>టెలిగ్రామ్‌కు అప్‌లోడ్ చేయబడుతోంది...</b>",
		Done:           "✅ <b>విజయవంతంగా డౌన్‌లోడ్ చేయబడింది!</b>",
		InvalidURL:     "⚠️ దయచేసి సరైన ఇన్‌స్టాగ్రామ్ లింక్ పంపండి.",
		RateLimited:    "⏳ దయచేసి కొద్దిసేపు వేచి ఉండి మళ్ళీ ప్రయత్నించండి.",
		DownloadError:  "❌ ఈ లింక్ నుండి మీడియా డౌన్‌లోడ్ కాలేదు.",
		UploadFallback: "❌ <b>టెలిగ్రామ్‌కు నేరుగా అప్‌లోడ్ కాలేదు.</b>\n\n🔗 <b>డైరెక్ట్ లింక్:</b>",
		BtnHelp:        "📖 సహాయం",
		BtnAbout:       "ℹ️ సమాచారం",
		BtnLanguage:    "🌐 భాష",
		BtnDirectLink:  "🔗 డైరెక్ట్ లింక్",
	},
	"ar": {
		LanguageName:   "العربية",
		Flag:           "🇸🇦",
		StartMessage:   "👋 <b>مرحبًا بك في بوت تحميل إنستغرام!</b>\n\nأرسل لي أي رابط إنستغرام (<b>ريلز، منشور، فيديو، صور</b>) وسأقوم بتحميله وإرساله فورًا!",
		HelpMessage:    "📖 <b>كيفية الاستخدام:</b>\n1. انسخ رابط المنشور أو الريلز من إنستغرام.\n2. أرسله إلى هذا البوت.\n3. سيتم إرسال الفيديو أو الصور لك مباشرة.",
		AboutMessage:   "ℹ️ <b>حول البوت:</b>\n• محرك Go فائق السرعة وخالي من السيرفرات\n• دعم متعدد لجهات التحميل\n• جودة أصلية ممتازة",
		SelectLang:     "🌐 <b>اختر لغتك المفضلة:</b>",
		LangUpdated:    "✅ تم تغيير اللغة إلى <b>العربية</b>!",
		ProcessingURL:  "🔎 <b>جارٍ فحص الرابط...</b>",
		ResolvingMedia: "⬇️ <b>جارٍ استخراج الوسائط...</b>",
		UploadingMedia: "⬆️ <b>جارٍ الرفع إلى تيليجرام...</b>",
		Done:           "✅ <b>تم التحميل بنجاح!</b>",
		InvalidURL:     "⚠️ يرجى إرسال رابط إنستغرام صالح.",
		RateLimited:    "⏳ طلبات كثيرة جدًا! يرجى الانتظار قليلاً.",
		DownloadError:  "❌ تعذر استخراج الوسائط من هذا الرابط.",
		UploadFallback: "❌ <b>تعذر الرفع المباشر إلى تيليجرام.</b>\n\n🔗 <b>رابط التحميل المباشر:</b>",
		BtnHelp:        "📖 مساعدة",
		BtnAbout:       "ℹ️ حول البوت",
		BtnLanguage:    "🌐 اللغة",
		BtnDirectLink:  "🔗 الرابط المباشر",
	},
	"ur": {
		LanguageName:   "اردو",
		Flag:           "🇵🇰",
		StartMessage:   "👋 <b>انسٹاگرام ڈاؤنلوڈر بوٹ میں خوش آمدید!</b>\n\nکوئی بھی انسٹاگرام لنک (<b>Reel, Post, Video, Photo</b>) بھیجیں اور فورا حاصل کریں!",
		HelpMessage:    "📖 <b>استعمال کا طریقہ:</b>\n1. انسٹاگرام ریل یا پوسٹ کا لنک کاپی کریں۔\n2. اس بوٹ کو بھیجیں۔\n3. میڈیا فورا آپ کو موصول ہو جائے گا۔",
		AboutMessage:   "ℹ️ <b>بوٹ کے بارے میں:</b>\n• تیز رفتار سرور لیس گو انجن\n• اعلی کوالٹی میڈیا ڈیلیوری",
		SelectLang:     "🌐 <b>اپنی زبان منتخب کریں:</b>",
		LangUpdated:    "✅ زبان <b>اردو</b> میں تبدیل کر دی گئی ہے!",
		ProcessingURL:  "🔎 <b>لنک چیک کیا جا رہا ہے...</b>",
		ResolvingMedia: "⬇️ <b>میڈیا حاصل کیا جا رہا ہے...</b>",
		UploadingMedia: "⬆️ <b>ٹیلیگرام پر اپ لوڈ ہو رہا ہے...</b>",
		Done:           "✅ <b>کامیابی سے ڈاؤن لوڈ ہو گیا!</b>",
		InvalidURL:     "⚠️ براہ کرم درست انسٹاگرام لنک بھیجیں۔",
		RateLimited:    "⏳ بہت زیادہ درخواستیں! تھوڑا انتظار کریں۔",
		DownloadError:  "❌ اس لنک سے میڈیا حاصل نہیں ہو سکا۔",
		UploadFallback: "❌ <b>ٹیلیگرام پر اپ لوڈ نہیں ہو سکا۔</b>\n\n🔗 <b>براہ راست ڈاؤن لوڈ لنک:</b>",
		BtnHelp:        "📖 مدد",
		BtnAbout:       "ℹ️ معلومات",
		BtnLanguage:    "🌐 زبان",
		BtnDirectLink:  "🔗 ڈائریکٹ لنک",
	},
	"bn": {
		LanguageName:   "বাংলা",
		Flag:           "🇧🇩",
		StartMessage:   "👋 <b>ইনস্টাগ্রাম ডাউনলোডার বটে স্বাগতম!</b>\n\nযেকোনো ইনস্টাগ্রাম লিংক (<b>Reel, Post, Video, Photo</b>) পাঠান, আমি তাৎক্ষণিকভাবে ডাউনলোড করে পাঠিয়ে দেব!",
		HelpMessage:    "📖 <b>ব্যবহার পদ্ধতি:</b>\n1. ইনস্টাগ্রাম রিল বা পোস্টের লিংক কপি করুন।\n2. বটে পাঠান।\n3. নিমিষেই মিডিয়া ফাইল পেয়ে যাবেন।",
		AboutMessage:   "ℹ️ <b>বট পরিচিতি:</b>\n• দ্রুততম সার্ভারলেস গো ইঞ্জিন\n• ফুল এইচডি মিডিয়া সাপোর্ট",
		SelectLang:     "🌐 <b>আপনার পছন্দের ভাষা নির্বাচন করুন:</b>",
		LangUpdated:    "✅ ভাষা পরিবর্তন করে <b>বাংলা</b> করা হয়েছে!",
		ProcessingURL:  "🔎 <b>লিংক পরীক্ষা করা হচ্ছে...</b>",
		ResolvingMedia: "⬇️ <b>মিডিয়া সংগ্রহ করা হচ্ছে...</b>",
		UploadingMedia: "⬆️ <b>টেলিগ্রামে আপলোড করা হচ্ছে...</b>",
		Done:           "✅ <b>সফলভাবে ডাউনলোড সম্পন্ন হয়েছে!</b>",
		InvalidURL:     "⚠️ দয়া করে একটি সঠিক ইনস্টাগ্রাম লিংক দিন।",
		RateLimited:    "⏳ অতিরিক্ত অনুরোধ! অনুগ্রহ করে কিছুক্ষণ অপেক্ষা করুন।",
		DownloadError:  "❌ এই লিংক থেকে মিডিয়া পাওয়া যায়নি।",
		UploadFallback: "❌ <b>টেলিগ্রামে সরাসরি আপলোড করা সম্ভব হয়নি।</b>\n\n🔗 <b>সরাসরি ডাউনলোড লিংক:</b>",
		BtnHelp:        "📖 সাহায্য",
		BtnAbout:       "ℹ️ তথ্য",
		BtnLanguage:    "🌐 ভাষা",
		BtnDirectLink:  "🔗 সরাসরি লিংক",
	},
	"zh": {
		LanguageName:   "中文",
		Flag:           "🇨🇳",
		StartMessage:   "👋 <b>欢迎使用 Instagram 下载机器人！</b>\n\n发送任何 Instagram 链接（<b>Reels、帖子、视频、图片、轮播图</b>），我将立即为您下载并发送！",
		HelpMessage:    "📖 <b>使用说明：</b>\n1. 复制 Instagram 视频或帖子链接。\n2. 发送给本机器人。\n3. 稍等片刻即可收到高清媒体文件。",
		AboutMessage:   "ℹ️ <b>关于本机器人：</b>\n• 基于 Go 的极速 Serverless 架构\n• 多节点智能故障转移\n• 原画质极速传输",
		SelectLang:     "🌐 <b>请选择您的偏好语言：</b>",
		LangUpdated:    "✅ 语言已切换为 <b>中文</b>！",
		ProcessingURL:  "🔎 <b>正在解析 Instagram 链接...</b>",
		ResolvingMedia: "⬇️ <b>正在提取媒体资源...</b>",
		UploadingMedia: "⬆️ <b>正在上传至 Telegram...</b>",
		Done:           "✅ <b>下载成功！</b>",
		InvalidURL:     "⚠️ 请发送有效的 Instagram 链接。",
		RateLimited:    "⏳ 请求过于频繁，请稍后再试。",
		DownloadError:  "❌ 无法从此链接提取媒体。",
		UploadFallback: "❌ <b>无法直接上传到 Telegram。</b>\n\n🔗 <b>直接下载链接：</b>",
		BtnHelp:        "📖 帮助",
		BtnAbout:       "ℹ️ 关于",
		BtnLanguage:    "🌐 切换语言",
		BtnDirectLink:  "🔗 原始链接",
	},
	"ru": {
		LanguageName:   "Русский",
		Flag:           "🇷🇺",
		StartMessage:   "👋 <b>Добро пожаловать в Instagram Downloader Bot!</b>\n\nОтправьте мне любую ссылку из Instagram (<b>Reel, Пост, Видео, Фото или Карусель</b>), и я моментально скачаю её для вас!",
		HelpMessage:    "📖 <b>Как использовать:</b>\n1. Скопируйте ссылку на Reel, видео или фото из Instagram.\n2. Отправьте ссылку этому боту.\n3. Получите файл прямо в чат!",
		AboutMessage:   "ℹ️ <b>О боте:</b>\n• Сверхбыстрый бессерверный движок на Go\n• Несколько резервных источников\n• Высокое качество загрузки",
		SelectLang:     "🌐 <b>Выберите язык интерфейса:</b>",
		LangUpdated:    "✅ Язык успешно изменен на <b>Русский</b>!",
		ProcessingURL:  "🔎 <b>Обработка ссылки...</b>",
		ResolvingMedia: "⬇️ <b>Извлечение видео/фото...</b>",
		UploadingMedia: "⬆️ <b>Загрузка файла в Telegram...</b>",
		Done:           "✅ <b>Успешно загружено!</b>",
		InvalidURL:     "⚠️ Пожалуйста, отправьте корректную ссылку Instagram.",
		RateLimited:    "⏳ Слишком много запросов! Подождите минуту.",
		DownloadError:  "❌ Не удалось извлечь медиа по этой ссылке.",
		UploadFallback: "❌ <b>Не удалось отправить файл напрямую в Telegram.</b>\n\n🔗 <b>Прямая ссылка для скачивания:</b>",
		BtnHelp:        "📖 Помощь",
		BtnAbout:       "ℹ️ О боте",
		BtnLanguage:    "🌐 Сменить язык",
		BtnDirectLink:  "🔗 Прямая ссылка",
	},
	"ko": {
		LanguageName:   "한국어",
		Flag:           "🇰🇷",
		StartMessage:   "👋 <b>인스타그램 다운로더 봇에 오신 것을 환영합니다!</b>\n\n인스타그램 링크(<b>릴스, 게시물, 동영상, 사진</b>)를 보내주시면 즉시 다운로드하여 전송해 드립니다!",
		HelpMessage:    "📖 <b>사용 방법:</b>\n1. 인스타그램 릴스 또는 게시물 링크를 복사합니다.\n2. 이 봇에 링크를 보냅니다.\n3. 잠시 후 미디어를 바로 확인하실 수 있습니다.",
		AboutMessage:   "ℹ️ <b>봇 정보:</b>\n• 초고속 서버리스 Go 엔진\n• 다양한 리졸버 백업 시스템\n• 최고 품질 미디어 전송",
		SelectLang:     "🌐 <b>선호하는 언어를 선택하세요:</b>",
		LangUpdated:    "✅ 언어가 <b>한국어</b>로 변경되었습니다!",
		ProcessingURL:  "🔎 <b>링크 확인 중...</b>",
		ResolvingMedia: "⬇️ <b>미디어 추출 중...</b>",
		UploadingMedia: "⬆️ <b>텔레그램으로 업로드 중...</b>",
		Done:           "✅ <b>다운로드 완료!</b>",
		InvalidURL:     "⚠️ 올바른 인스타그램 URL을 입력해 주세요.",
		RateLimited:    "⏳ 요청이 너무 많습니다. 잠시 후 다시 시도해 주세요.",
		DownloadError:  "❌ 해당 링크에서 미디어를 가져올 수 없습니다.",
		UploadFallback: "❌ <b>텔레그램에 직접 업로드할 수 없습니다.</b>\n\n🔗 <b>직접 다운로드 링크:</b>",
		BtnHelp:        "📖 도움말",
		BtnAbout:       "ℹ️ 정보",
		BtnLanguage:    "🌐 언어 설정",
		BtnDirectLink:  "🔗 직접 링크",
	},
	"ja": {
		LanguageName:   "日本語",
		Flag:           "🇯🇵",
		StartMessage:   "👋 <b>Instagram ダウンローダーボットへようこそ！</b>\n\nInstagram のリンク（<b>リール、投稿、動画、写真、カルーセル</b>）を送信すると、すぐにダウンロードしてお届けします！",
		HelpMessage:    "📖 <b>使い方：</b>\n1. Instagram の動画や投稿のリンクをコピーします。\n2. このボットに送信します。\n3. 高画質の動画・画像が直接届きます。",
		AboutMessage:   "ℹ️ <b>ボットについて：</b>\n• 超高速サーバーレス Go エンジン\n• 複数のリゾルバー自動フォールバック\n• 安全で永続ストレージ不要",
		SelectLang:     "🌐 <b>使用する言語を選択してください：</b>",
		LangUpdated:    "✅ 言語を <b>日本語</b> に変更しました！",
		ProcessingURL:  "🔎 <b>リンクを確認しています...</b>",
		ResolvingMedia: "⬇️ <b>メディアを抽出中...</b>",
		UploadingMedia: "⬆️ <b>Telegram へアップロード中...</b>",
		Done:           "✅ <b>ダウンロードが完了しました！</b>",
		InvalidURL:     "⚠️ 有効な Instagram のリンクを送信してください。",
		RateLimited:    "⏳ リクエストが多すぎます。しばらくお待ちください。",
		DownloadError:  "❌ このリンクからメディアを取得できませんでした。",
		UploadFallback: "❌ <b>Telegram への直接アップロードに失敗しました。</b>\n\n🔗 <b>直接ダウンロードリンク：</b>",
		BtnHelp:        "📖 ヘルプ",
		BtnAbout:       "ℹ️ ボット情報",
		BtnLanguage:    "🌐 言語変更",
		BtnDirectLink:  "🔗 ダウンロードリンク",
	},
	"tr": {
		LanguageName:   "Türkçe",
		Flag:           "🇹🇷",
		StartMessage:   "👋 <b>Instagram İndirici Botuna Hoş Geldiniz!</b>\n\nHerhangi bir Instagram bağlantısını (<b>Reel, Gönderi, Video, Fotoğraf veya Carousel</b>) gönderin, anında indirip size ulaştırayım!",
		HelpMessage:    "📖 <b>Nasıl Kullanılır:</b>\n1. Instagram Reel veya gönderi linkini kopyalayın.\n2. Bu bota gönderin.\n3. Medya hemen sohbetinize yüklenecektir.",
		AboutMessage:   "ℹ️ <b>Bot Hakkında:</b>\n• Ultra hızlı Sunucusuz Go Motoru\n• Çoklu Otomatik Yedek Çözücüler\n• Orijinal Yüksek Kalite",
		SelectLang:     "🌐 <b>Tercih ettiğiniz dili seçin:</b>",
		LangUpdated:    "✅ Dil <b>Türkçe</b> olarak güncellendi!",
		ProcessingURL:  "🔎 <b>Bağlantı işleniyor...</b>",
		ResolvingMedia: "⬇️ <b>Medya ayrıştırılıyor...</b>",
		UploadingMedia: "⬆️ <b>Telegram'a yükleniyor...</b>",
		Done:           "✅ <b>Başarıyla indirildi!</b>",
		InvalidURL:     "⚠️ Lütfen geçerli bir Instagram bağlantısı gönderin.",
		RateLimited:    "⏳ Çok fazla istek! Lütfen biraz bekleyin.",
		DownloadError:  "❌ Bu bağlantıdan medya alınamadı.",
		UploadFallback: "❌ <b>Medya doğrudan Telegram'a yüklenemedi.</b>\n\n🔗 <b>Doğrudan indirme bağlantısı:</b>",
		BtnHelp:        "📖 Yardım",
		BtnAbout:       "ℹ️ Bilgi",
		BtnLanguage:    "🌐 Dil Değiştir",
		BtnDirectLink:  "🔗 Doğrudan Bağlantı",
	},
}

// Get returns the TextBundle for the given language code, falling back to English.
func Get(langCode string) TextBundle {
	if b, ok := Translations[langCode]; ok {
		return b
	}
	return Translations["en"]
}

// ForUser returns the TextBundle for the specified Telegram user ID.
func ForUser(userID int64) TextBundle {
	code := GlobalUserLangStore.Get(userID)
	return Get(code)
}
