import { Link } from '@tanstack/react-router'

const STAGES = [
  { stage: 1, label: '불씨', color: 'var(--color-accent)', emoji: '🕯️', desc: '작은 불씨가 피어납니다. 은은한 빛이 감돌아요.' },
  { stage: 2, label: '모닥불', color: 'var(--color-warning)', emoji: '🔥', desc: '불꽃이 커지고 따뜻한 빛이 퍼져나갑니다.' },
  { stage: 3, label: '불기둥', color: '#e4531b', emoji: '🔥', desc: '거대한 불기둥이 솟아오르며 화면이 뜨거워집니다.' },
  { stage: 4, label: '불바다', color: 'var(--color-negative)', emoji: '🚒', desc: '불바다가 됩니다. 소방차가 출동해요!' },
  { stage: 5, label: '불지옥 MAX', color: '#ff0040', emoji: '💀', desc: '최고 단계! 폭발과 함께 불이 옆으로 번집니다.' },
]

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mb-8">
      <h2 className="text-[1rem] font-bold text-[var(--color-text-base)] mb-3">{title}</h2>
      {children}
    </section>
  )
}

function Card({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`bg-[var(--color-bg-elevated)] rounded-[12px] p-4 ${className}`}>
      {children}
    </div>
  )
}

export function GuidePage() {
  return (
    <div className="min-h-svh bg-[var(--color-bg-base)]">
      {/* Header */}
      <header className="sticky top-0 z-10 flex items-center h-11 px-3 bg-[var(--color-bg-surface)] border-b border-[var(--color-bg-elevated)]">
        <Link to="/map" className="flex items-center gap-2 text-[var(--color-text-secondary)]">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M19 12H5M12 19l-7-7 7-7" />
          </svg>
          <span className="text-[0.8125rem] font-medium">지도로 돌아가기</span>
        </Link>
      </header>

      <div className="px-5 py-6 pb-16">
        {/* Title */}
        <div className="mb-8">
          <h1 className="text-[1.5rem] font-extrabold text-[var(--color-text-base)] leading-tight">
            이용 안내
          </h1>
          <p className="text-[0.8125rem] text-[var(--color-text-secondary)] mt-1">
            화르르 서비스를 200% 즐기는 방법
          </p>
        </div>

        {/* 서비스 소개 */}
        <Section title="화르르가 뭔가요?">
          <Card>
            <p className="text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed">
              화르르는 <strong className="text-[var(--color-text-base)]">GPS 기반 실시간 가상 불 지도</strong>입니다.
              내 위치에 가상의 성냥을 던져 불을 피우고, 전국에서 동시에 타오르는 불꽃 현황을 실시간으로 확인할 수 있어요.
            </p>
            <p className="text-[0.8125rem] text-[var(--color-text-secondary)] leading-relaxed mt-2">
              모든 불꽃은 <strong className="text-[var(--color-negative)]">100% 가상</strong>이며,
              실제 화재 · 재난 정보와는 관련이 없습니다.
            </p>
          </Card>
        </Section>

        {/* 불 지르기 */}
        <Section title="불 지르는 방법">
          <div className="space-y-3">
            <Card>
              <div className="flex items-start gap-3">
                <div className="w-8 h-8 rounded-[10px] bg-[#e4531b]/15 flex items-center justify-center shrink-0 mt-0.5">
                  <span className="text-[0.875rem]">👆</span>
                </div>
                <div>
                  <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">1. 하단 버튼을 탭하세요</p>
                  <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                    화면 아래의 주황색 "실시간 내 위치에 불지르기" 버튼을 누르면 성냥이 날아갑니다.
                  </p>
                </div>
              </div>
            </Card>
            <Card>
              <div className="flex items-start gap-3">
                <div className="w-8 h-8 rounded-[10px] bg-[#e4531b]/15 flex items-center justify-center shrink-0 mt-0.5">
                  <span className="text-[0.875rem]">🔥</span>
                </div>
                <div>
                  <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">2. 성냥이 떨어지면 불이 붙어요</p>
                  <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                    성냥은 포물선을 그리며 내 위치에 떨어지고, 착지하면 불꽃이 피어납니다.
                    계속 탭할수록 불이 점점 커져요!
                  </p>
                </div>
              </div>
            </Card>
            <Card>
              <div className="flex items-start gap-3">
                <div className="w-8 h-8 rounded-[10px] bg-[#e4531b]/15 flex items-center justify-center shrink-0 mt-0.5">
                  <span className="text-[0.875rem]">👥</span>
                </div>
                <div>
                  <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">3. 다른 사람과 함께 불을 키울 수 있어요</p>
                  <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                    같은 위치에 있는 다른 사용자들이 함께 불을 던지면 더 빠르게 불이 커집니다.
                    실시간으로 접속자 수를 확인할 수 있어요.
                  </p>
                </div>
              </div>
            </Card>
          </div>
        </Section>

        {/* 화재 단계 */}
        <Section title="화재 5단계">
          <p className="text-[0.75rem] text-[var(--color-text-secondary)] mb-3 leading-relaxed">
            성냥을 계속 던질수록 불이 성장합니다. 각 단계마다 시각 효과가 달라져요.
          </p>
          <div className="space-y-2">
            {STAGES.map(({ stage, label, color, emoji, desc }) => (
              <Card key={stage} className="flex items-center gap-3">
                <div
                  className="w-10 h-10 rounded-[10px] flex items-center justify-center shrink-0 text-[1.25rem]"
                  style={{ backgroundColor: `color-mix(in srgb, ${color} 15%, transparent)` }}
                >
                  {emoji}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[0.8125rem] font-bold" style={{ color }}>{stage}단계</span>
                    <span className="text-[0.75rem] text-[var(--color-text-secondary)]">{label}</span>
                  </div>
                  <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-0.5 leading-relaxed">{desc}</p>
                </div>
              </Card>
            ))}
          </div>
        </Section>

        {/* MAX 불 번짐 */}
        <Section title="MAX 상태에서 불이 번져요">
          <Card>
            <div className="flex items-center gap-2 mb-2">
              <span className="text-[1rem]">💥</span>
              <p className="text-[0.8125rem] font-bold text-[#ff0040]">MAX 단계 도달 후 계속 탭하면?</p>
            </div>
            <p className="text-[0.75rem] text-[var(--color-text-secondary)] leading-relaxed">
              5단계(MAX · 불지옥)에서 계속 성냥을 던지면, 불이 더 이상 커지지 않는 대신
              <strong className="text-[var(--color-text-base)]"> 주변 지역으로 불이 번져나갑니다</strong>.
            </p>
            <p className="text-[0.75rem] text-[var(--color-text-secondary)] leading-relaxed mt-2">
              불씨가 포물선을 그리며 인접한 격자로 날아가는 궤적 애니메이션이 재생되고,
              도착 지점에 새로운 불이 점화됩니다.
              전국을 불바다로 만들어보세요!
            </p>
          </Card>
        </Section>

        {/* 소방차 */}
        <Section title="소방차 출동">
          <Card>
            <div className="flex items-start gap-3">
              <span className="text-[1.5rem] shrink-0">🚒</span>
              <div>
                <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">4단계 이상이면 소방차가 출동합니다</p>
                <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                  불이 4단계(불바다) 이상으로 커지면, 해당 위치에 소방차 마커가 나타납니다.
                  경광등이 번쩍이며 좌우로 흔들리는 모습을 줌 레벨 18 이상에서 확인할 수 있어요.
                </p>
                <p className="text-[0.6875rem] text-[var(--color-text-secondary)] mt-1 italic">
                  * 소방차는 시각적 연출이며, 불을 끄지는 않습니다.
                </p>
              </div>
            </div>
          </Card>
        </Section>

        {/* 실시간 채팅 */}
        <Section title="실시간 채팅">
          <Card>
            <div className="flex items-start gap-3">
              <span className="text-[1.5rem] shrink-0">💬</span>
              <div>
                <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">전국 사용자와 실시간 대화</p>
                <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                  지도 우측 하단의 말풍선 버튼을 눌러 채팅에 참여하세요.
                  익명으로 자동 입장되며, 메시지는 1시간 후 자동 삭제됩니다.
                </p>
              </div>
            </div>
          </Card>
        </Section>

        {/* 다른 지역 구경하기 */}
        <Section title="다른 화재 지역 구경하기">
          <Card>
            <div className="flex items-start gap-3">
              <span className="text-[1.5rem] shrink-0">📍</span>
              <div>
                <p className="text-[0.8125rem] font-bold text-[var(--color-text-base)]">실시간 화재 지역을 탐방하세요</p>
                <p className="text-[0.75rem] text-[var(--color-text-secondary)] mt-1 leading-relaxed">
                  하단 패널에서 "실시간 화재 지역" 숫자를 탭하면,
                  현재 불이 타고 있는 랜덤 지역으로 카메라가 날아갑니다.
                  전국 각지의 불꽃 현황을 둘러보세요.
                </p>
              </div>
            </div>
          </Card>
        </Section>

        {/* 위치 권한 */}
        <Section title="위치 권한 안내">
          <Card>
            <p className="text-[0.75rem] text-[var(--color-text-secondary)] leading-relaxed">
              불을 지르려면 <strong className="text-[var(--color-text-base)]">GPS 위치 권한</strong>이 필요합니다.
              권한을 거부하면 지도를 구경할 수는 있지만, 직접 불을 지를 수는 없어요.
            </p>
            <div className="mt-3 space-y-2">
              <div className="flex items-start gap-2">
                <span className="text-[0.75rem] shrink-0">📱</span>
                <p className="text-[0.6875rem] text-[var(--color-text-secondary)] leading-relaxed">
                  <strong className="text-[var(--color-text-base)]">iPhone</strong>:
                  설정 → 개인정보 보호 및 보안 → 위치 서비스 → Safari → 허용
                </p>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-[0.75rem] shrink-0">🤖</span>
                <p className="text-[0.6875rem] text-[var(--color-text-secondary)] leading-relaxed">
                  <strong className="text-[var(--color-text-base)]">Android</strong>:
                  주소창 자물쇠 아이콘 → 사이트 설정 → 위치 → 허용
                </p>
              </div>
            </div>
          </Card>
        </Section>

        {/* CTA */}
        <Link
          to="/map"
          className="block w-full rounded-[14px] bg-[#e4531b] py-3.5 text-center text-[0.9375rem] font-bold text-white shadow-[var(--shadow-medium)] transition-transform active:scale-[0.96]"
        >
          지도로 돌아가기
        </Link>
      </div>
    </div>
  )
}
