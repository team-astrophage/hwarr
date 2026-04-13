'use strict'

const DEFAULTS = {
  server: process.env.SERVER_URL || 'http://localhost:8000',
  users: 200,
  durationSec: 300,
  rampMs: 250,
  minIntervalMs: 200,
  maxIntervalMs: 500,
  reportIntervalMs: 5000,
}

// Nationwide real landmarks across all provinces. Balanced distribution so
// 200 virtual users spread out instead of piling onto Seoul. Every point is a
// known landmark/station/park on land — safe to feed to fire:ignite without
// landing in the sea or DPRK. Grouped by region for readability only; order
// doesn't affect round-robin assignment.
const CITIES = [
  // --- 서울 (25개 자치구 커버) ---
  { id: 'gwanghwamun', name: '광화문광장', lat: 37.576, lng: 126.9769 }, // 종로
  { id: 'myeongdong', name: '명동', lat: 37.5636, lng: 126.9834 }, // 중구
  { id: 'namsan', name: '남산서울타워', lat: 37.5512, lng: 126.9882 }, // 용산(중구 경계)
  { id: 'itaewon', name: '이태원', lat: 37.5345, lng: 126.9946 }, // 용산
  { id: 'seoul_station', name: '서울역', lat: 37.5547, lng: 126.9706 }, // 중구/용산
  { id: 'ddp', name: '동대문디자인플라자', lat: 37.5665, lng: 127.0092 }, // 중구
  { id: 'gangnam', name: '강남역', lat: 37.4979, lng: 127.0276 }, // 강남
  { id: 'coex', name: '코엑스', lat: 37.5126, lng: 127.059 }, // 강남
  { id: 'apgujeong', name: '압구정로데오', lat: 37.5274, lng: 127.0395 }, // 강남
  { id: 'hongdae', name: '홍대입구', lat: 37.5563, lng: 126.9236 }, // 마포
  { id: 'sangam_dmc', name: '상암 DMC', lat: 37.5779, lng: 126.8909 }, // 마포
  { id: 'yeouido', name: '여의도공원', lat: 37.5284, lng: 126.9344 }, // 영등포
  { id: 'jamsil', name: '잠실종합운동장', lat: 37.5153, lng: 127.0728 }, // 송파
  { id: 'olympic_park', name: '올림픽공원', lat: 37.5202, lng: 127.1215 }, // 송파
  { id: 'seongsu', name: '성수동 카페거리', lat: 37.5446, lng: 127.0558 }, // 성동
  { id: 'wangsimni', name: '왕십리역', lat: 37.5612, lng: 127.0377 }, // 성동
  { id: 'konkuk', name: '건대입구', lat: 37.5403, lng: 127.0697 }, // 광진
  { id: 'sinchon', name: '신촌역', lat: 37.5555, lng: 126.9368 }, // 서대문
  { id: 'snu_ipgu', name: '서울대입구', lat: 37.4813, lng: 126.9527 }, // 관악
  { id: 'sadang', name: '사당역', lat: 37.4766, lng: 126.9816 }, // 동작
  { id: 'express_bus', name: '고속터미널', lat: 37.5049, lng: 127.0047 }, // 서초
  { id: 'mok_dong', name: '목동 현대백화점', lat: 37.5266, lng: 126.8756 }, // 양천
  { id: 'guro_digital', name: '구로디지털단지', lat: 37.4852, lng: 126.9013 }, // 구로
  { id: 'gimpo_airport', name: '김포공항', lat: 37.5583, lng: 126.7906 }, // 강서
  { id: 'nowon', name: '노원역', lat: 37.6541, lng: 127.0615 }, // 노원
  { id: 'suyu', name: '수유역', lat: 37.6377, lng: 127.0254 }, // 강북
  { id: 'eunpyeong', name: '연신내역', lat: 37.619, lng: 126.9213 }, // 은평

  // --- 인천 (7) ---
  { id: 'incheon_songdo', name: '송도센트럴파크', lat: 37.3925, lng: 126.6632 }, // 연수
  { id: 'incheon_airport', name: '인천공항', lat: 37.4602, lng: 126.4407 }, // 중구(영종)
  { id: 'incheon_wolmi', name: '월미도', lat: 37.4757, lng: 126.597 }, // 중구
  { id: 'incheon_bupyeong', name: '부평역', lat: 37.4898, lng: 126.7248 }, // 부평
  { id: 'incheon_terminal', name: '인천종합터미널', lat: 37.4418, lng: 126.7006 }, // 미추홀
  { id: 'incheon_cheongna', name: '청라국제도시', lat: 37.5283, lng: 126.6574 }, // 서구
  { id: 'ganghwa', name: '강화도 고려산', lat: 37.7725, lng: 126.4414 }, // 강화

  // --- 경기 (8) ---
  { id: 'suwon_hwaseong', name: '수원화성', lat: 37.287, lng: 127.0095 },
  { id: 'seongnam_pangyo', name: '판교역', lat: 37.3947, lng: 127.1112 },
  { id: 'yongin_everland', name: '에버랜드', lat: 37.2949, lng: 127.2022 },
  { id: 'goyang_ilsan', name: '일산호수공원', lat: 37.6584, lng: 126.7709 },
  { id: 'bucheon', name: '부천시청', lat: 37.5035, lng: 126.766 },
  { id: 'ansan', name: '안산중앙역', lat: 37.3217, lng: 126.8309 },
  { id: 'pyeongtaek', name: '평택역', lat: 36.9897, lng: 127.0855 },
  { id: 'gwacheon', name: '과천 서울대공원', lat: 37.4353, lng: 127.0153 },

  // --- 강원 (6) ---
  { id: 'chuncheon', name: '춘천 남이섬', lat: 37.7905, lng: 127.5257 },
  { id: 'wonju', name: '원주시청', lat: 37.3422, lng: 127.9202 },
  { id: 'gangneung', name: '강릉 경포대', lat: 37.7955, lng: 128.9058 },
  { id: 'sokcho', name: '속초 중앙시장', lat: 38.2078, lng: 128.591 },
  { id: 'donghae', name: '동해역', lat: 37.5244, lng: 129.1165 },
  { id: 'pyeongchang', name: '평창 알펜시아', lat: 37.6634, lng: 128.6809 },

  // --- 충청 (8) ---
  { id: 'daejeon_dunsan', name: '대전 둔산동', lat: 36.3511, lng: 127.3782 },
  { id: 'daejeon_expo', name: '대전 엑스포공원', lat: 36.3745, lng: 127.3881 },
  { id: 'sejong', name: '세종정부청사', lat: 36.5041, lng: 127.2621 },
  { id: 'cheonan', name: '천안아산역', lat: 36.7946, lng: 127.1045 },
  { id: 'cheongju', name: '청주시청', lat: 36.6424, lng: 127.489 },
  { id: 'chungju', name: '충주시청', lat: 36.9911, lng: 127.9259 },
  { id: 'asan', name: '아산 온양온천', lat: 36.7823, lng: 127.0043 },
  { id: 'gongju', name: '공주 공산성', lat: 36.4607, lng: 127.1192 },

  // --- 전라 (10) ---
  { id: 'gwangju_chungjang', name: '광주 충장로', lat: 35.1488, lng: 126.9156 },
  { id: 'gwangju_sangmu', name: '광주 상무지구', lat: 35.1511, lng: 126.8416 },
  { id: 'jeonju_hanok', name: '전주 한옥마을', lat: 35.8152, lng: 127.153 },
  { id: 'gunsan', name: '군산 근대문화거리', lat: 35.9855, lng: 126.7117 },
  { id: 'iksan', name: '익산역', lat: 35.9419, lng: 126.9597 },
  { id: 'mokpo', name: '목포역', lat: 34.7911, lng: 126.3884 },
  { id: 'yeosu', name: '여수 엑스포', lat: 34.7482, lng: 127.7472 },
  { id: 'suncheon', name: '순천만습지', lat: 34.8834, lng: 127.5071 },
  { id: 'namwon', name: '남원 광한루', lat: 35.4048, lng: 127.3808 },
  { id: 'gwangyang', name: '광양시청', lat: 34.9406, lng: 127.6959 },

  // --- 경상 (14) ---
  { id: 'daegu_dongseong', name: '대구 동성로', lat: 35.8691, lng: 128.5958 },
  { id: 'daegu_suseong', name: '대구 수성못', lat: 35.8278, lng: 128.6254 },
  { id: 'busan_haeundae', name: '부산 해운대', lat: 35.1587, lng: 129.1604 },
  { id: 'busan_seomyeon', name: '부산 서면', lat: 35.1579, lng: 129.0594 },
  { id: 'busan_gwangalli', name: '부산 광안리', lat: 35.1531, lng: 129.1189 },
  { id: 'ulsan', name: '울산 태화강공원', lat: 35.5535, lng: 129.3182 },
  { id: 'changwon', name: '창원 시청', lat: 35.228, lng: 128.6811 },
  { id: 'pohang', name: '포항 영일대', lat: 36.0512, lng: 129.3772 },
  { id: 'gyeongju_bulguksa', name: '경주 불국사', lat: 35.79, lng: 129.3322 },
  { id: 'gyeongju_daereungwon', name: '경주 대릉원', lat: 35.8378, lng: 129.2127 },
  { id: 'jinju', name: '진주성', lat: 35.1898, lng: 128.0808 },
  { id: 'tongyeong', name: '통영 동피랑', lat: 34.8459, lng: 128.4299 },
  { id: 'gimhae', name: '김해시청', lat: 35.2285, lng: 128.8894 },
  { id: 'andong', name: '안동 하회마을', lat: 36.5384, lng: 128.5208 },

  // --- 제주 (4) ---
  { id: 'jeju_city', name: '제주시청', lat: 33.4996, lng: 126.5312 },
  { id: 'seogwipo', name: '서귀포시청', lat: 33.2541, lng: 126.5601 },
  { id: 'jeju_jungmun', name: '제주 중문관광단지', lat: 33.2478, lng: 126.4119 },
  { id: 'jeju_seongsan', name: '성산일출봉', lat: 33.4586, lng: 126.9425 },
]

// ~1km ≈ 0.009° lat. Fire jitter = ~1km radius so fires cluster in a
// recognizable neighborhood rather than stacking on a single grid cell.
const FIRE_JITTER_DEG = 0.009
// Viewport half-size ~5km so each user watches their own city region and we
// avoid the nation-wide bbox that triggers the sparse fallback / OOM path.
const VIEWPORT_HALF_DEG = 0.045

function randFloat(min, max) {
  return min + Math.random() * (max - min)
}

function randInt(min, max) {
  return Math.floor(randFloat(min, max + 1))
}

// Assigns a city to virtual user `index` in round-robin fashion so the load
// spreads evenly across all landmarks even at small --users counts.
function cityForUser(index) {
  return CITIES[index % CITIES.length]
}

// Random coordinate within ~1km of the given city.
function fireNearCity(city) {
  return {
    lat: city.lat + randFloat(-FIRE_JITTER_DEG, FIRE_JITTER_DEG),
    lng: city.lng + randFloat(-FIRE_JITTER_DEG, FIRE_JITTER_DEG),
  }
}

// Viewport bbox centered on the user's city (~5km half-size).
function viewportForCity(city) {
  return {
    neLat: city.lat + VIEWPORT_HALF_DEG,
    neLng: city.lng + VIEWPORT_HALF_DEG,
    swLat: city.lat - VIEWPORT_HALF_DEG,
    swLng: city.lng - VIEWPORT_HALF_DEG,
  }
}

function parseArgs(argv) {
  const out = { ...DEFAULTS }
  for (let i = 0; i < argv.length; i++) {
    const key = argv[i]
    const val = argv[i + 1]
    switch (key) {
      case '--users':
        out.users = parseInt(val, 10)
        i++
        break
      case '--duration-sec':
        out.durationSec = parseInt(val, 10)
        i++
        break
      case '--ramp-ms':
        out.rampMs = parseInt(val, 10)
        i++
        break
      case '--server':
        out.server = val
        i++
        break
      case '--min-interval-ms':
        out.minIntervalMs = parseInt(val, 10)
        i++
        break
      case '--max-interval-ms':
        out.maxIntervalMs = parseInt(val, 10)
        i++
        break
      case '-h':
      case '--help':
        out.help = true
        break
    }
  }
  return out
}

function helpText() {
  return `hwarr loadtest — Socket.IO multi-user fire simulator

Usage: node run.js [options]

Options:
  --users N              concurrent virtual users (default ${DEFAULTS.users})
  --duration-sec N       total run duration in seconds (default ${DEFAULTS.durationSec})
  --ramp-ms N            delay between each user connect during ramp-up (default ${DEFAULTS.rampMs})
  --server URL           server base URL (default ${DEFAULTS.server})
  --min-interval-ms N    min gap between fire:ignite per user (default ${DEFAULTS.minIntervalMs})
  --max-interval-ms N    max gap between fire:ignite per user (default ${DEFAULTS.maxIntervalMs})
  -h, --help             show this help
`
}

module.exports = {
  DEFAULTS,
  CITIES,
  randFloat,
  randInt,
  cityForUser,
  fireNearCity,
  viewportForCity,
  parseArgs,
  helpText,
}
