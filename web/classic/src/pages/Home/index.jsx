/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useContext, useEffect, useState } from 'react';
import { Button, Typography } from '@douyinfe/semi-ui';
import { API, showError, copy, showSuccess } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { StatusContext } from '../../context/Status';
import { useActualTheme } from '../../context/Theme';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import {
  KeyRound,
  BookOpen,
  Github,
  Copy,
  Check,
  Network,
  Plug,
  Gauge,
  ShieldCheck,
  Wallet,
} from 'lucide-react';
import NoticeModal from '../../components/layout/NoticeModal';
import {
  Moonshot,
  OpenAI,
  Claude,
  Gemini,
  Meta,
  Mistral,
  Grok,
  Azure,
  DeepSeek,
  Qwen,
  Yi,
  Stepfun,
  Baichuan,
  Ai360,
  Minimax,
} from '@lobehub/icons';

const { Text } = Typography;

// 主体接入端点路径（与 base URL 拼接成完整 endpoint）
const ENDPOINT_PATH = '/v1/chat/completions';

// 供应商展示网格：品牌名为标识符，刻意不翻译；图标来自 @lobehub/icons。
// Featured 大卡：四家头部供应商，分别使用品牌色 highlight + 兼容协议 pill。
const FEATURED_PROVIDERS = [
  {
    name: 'OpenAI',
    icon: <OpenAI size={64} />,
    compatLabel: '兼容 OpenAI 接口',
    accentClass: 'home-provider-featured-card--openai',
  },
  {
    name: 'Claude',
    icon: <Claude.Color size={64} />,
    compatLabel: '兼容 Anthropic 接口',
    accentClass: 'home-provider-featured-card--claude',
  },
  {
    name: 'Google Gemini',
    icon: <Gemini.Color size={64} />,
    compatLabel: '兼容 Gemini API',
    accentClass: 'home-provider-featured-card--gemini',
  },
  {
    name: 'DeepSeek',
    icon: <DeepSeek.Color size={64} />,
    compatLabel: '兼容 DeepSeek API',
    accentClass: 'home-provider-featured-card--deepseek',
  },
];

// 二级小卡：6 列 × 2 行；30+ 占位卡单独渲染。
const SECONDARY_PROVIDERS = [
  { name: 'Meta Llama', icon: <Meta.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Mistral AI', icon: <Mistral.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'xAI Grok', icon: <Grok size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Azure OpenAI', icon: <Azure.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Moonshot AI', icon: <Moonshot size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Qwen', icon: <Qwen.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Yi', icon: <Yi.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'MiniMax', icon: <Minimax.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'StepFun', icon: <Stepfun.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: 'Baichuan', icon: <Baichuan.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
  { name: '360智脑', icon: <Ai360.Color size={22} />, compatLabel: '兼容 OpenAI 接口' },
];

const Home = () => {
  const { t, i18n } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const actualTheme = useActualTheme();
  const [homePageContentLoaded, setHomePageContentLoaded] = useState(false);
  const [homePageContent, setHomePageContent] = useState('');
  const [noticeVisible, setNoticeVisible] = useState(false);
  const [copied, setCopied] = useState(false);
  const isMobile = useIsMobile();
  const isDemoSiteMode = statusState?.status?.demo_site_enabled || false;
  const docsLink =
    statusState?.status?.docs_link || 'https://docs.newapi.pro';
  const serverAddress =
    statusState?.status?.server_address || `${window.location.origin}`;
  const fullEndpoint = `${serverAddress}${ENDPOINT_PATH}`;

  // 首屏五项功能条：标题 + 描述，图标使用统一的中性强调色，明暗一致
  const features = [
    {
      icon: Network,
      title: t('统一接入'),
      desc: t('一个 Endpoint 接入 40+ 上游供应商'),
      color: '#3b82f6',
    },
    {
      icon: Plug,
      title: t('兼容 OpenAI API'),
      desc: t('直接兼容主流 SDK'),
      color: '#06b6d4',
    },
    {
      icon: Gauge,
      title: t('快速稳定'),
      desc: t('低延迟路由与负载均衡'),
      color: '#10b981',
    },
    {
      icon: ShieldCheck,
      title: t('安全可靠'),
      desc: t('细粒度密钥与访问控制'),
      color: '#8b5cf6',
    },
    {
      icon: Wallet,
      title: t('高性价比'),
      desc: t('透明的按量计费'),
      color: '#f59e0b',
    },
  ];

  const displayHomePageContent = async () => {
    setHomePageContent(localStorage.getItem('home_page_content') || '');
    const res = await API.get('/api/home_page_content');
    const { success, message, data } = res.data;
    if (success) {
      let content = data;
      if (!data.startsWith('https://')) {
        content = marked.parse(data);
      }
      setHomePageContent(content);
      localStorage.setItem('home_page_content', content);

      // 如果内容是 URL，则发送主题模式
      if (data.startsWith('https://')) {
        const iframe = document.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            iframe.contentWindow.postMessage({ themeMode: actualTheme }, '*');
            iframe.contentWindow.postMessage({ lang: i18n.language }, '*');
          };
        }
      }
    } else {
      showError(message);
      setHomePageContent('加载首页内容失败...');
    }
    setHomePageContentLoaded(true);
  };

  const handleCopyEndpoint = async () => {
    const ok = await copy(fullEndpoint);
    if (ok) {
      setCopied(true);
      showSuccess(t('已复制到剪切板'));
      setTimeout(() => setCopied(false), 2000);
    }
  };

  useEffect(() => {
    const checkNoticeAndShow = async () => {
      const lastCloseDate = localStorage.getItem('notice_close_date');
      const today = new Date().toDateString();
      if (lastCloseDate !== today) {
        try {
          const res = await API.get('/api/notice');
          const { success, data } = res.data;
          if (success && data && data.trim() !== '') {
            setNoticeVisible(true);
          }
        } catch (error) {
          console.error('获取公告失败:', error);
        }
      }
    };

    checkNoticeAndShow();
  }, []);

  useEffect(() => {
    displayHomePageContent().then();
  }, []);

  return (
    <div className='classic-page-fill classic-home-page w-full overflow-x-hidden'>
      <NoticeModal
        visible={noticeVisible}
        onClose={() => setNoticeVisible(false)}
        isMobile={isMobile}
      />
      {homePageContentLoaded && homePageContent === '' ? (
        <div className='home-tech w-full overflow-x-hidden'>
          {/* ==================== 首屏 Hero ==================== */}
          <section className='home-hero-section relative overflow-hidden border-b border-semi-color-border'>
            {/* 暗色技术风背景：径向辉光 + 网格 + 点阵 + 代码片段，明暗仅颜色不同 */}
            <div className='home-tech-bg' aria-hidden='true'>
              <div className='home-tech-glow' />
              <div className='home-tech-grid' />
              <div className='home-tech-dots' />
              <div className='home-tech-code' aria-hidden='true'>
                <div className='home-code-line'><span className='home-code-no'>01</span><span><span className='home-code-method'>POST</span>{' '}<span className='home-code-url'>{fullEndpoint}</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>02</span><span><span className='home-code-key'>Content-Type</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-string'>application/json</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>03</span><span><span className='home-code-key'>Authorization</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-string'>Bearer sk-*****</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>04</span><span /></div>
                <div className='home-code-line'><span className='home-code-no'>05</span><span><span className='home-code-punc'>{'{'}</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>06</span><span>{'  '}<span className='home-code-key'>"model"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-string'>"gpt-4o"</span><span className='home-code-punc'>,</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>07</span><span>{'  '}<span className='home-code-key'>"messages"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-punc'>[</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>08</span><span>{'    '}<span className='home-code-punc'>{'{'}</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>09</span><span>{'      '}<span className='home-code-key'>"role"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-string'>"user"</span><span className='home-code-punc'>,</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>10</span><span>{'      '}<span className='home-code-key'>"content"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-string'>"{t('你好，请介绍一下你自己。')}"</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>11</span><span>{'    '}<span className='home-code-punc'>{'}'}</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>12</span><span>{'  '}<span className='home-code-punc'>],</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>13</span><span>{'  '}<span className='home-code-key'>"stream"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-value'>false</span><span className='home-code-punc'>,</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>14</span><span>{'  '}<span className='home-code-key'>"temperature"</span><span className='home-code-punc'>:</span>{' '}<span className='home-code-value'>0.7</span></span></div>
                <div className='home-code-line'><span className='home-code-no'>15</span><span><span className='home-code-punc'>{'}'}</span></span></div>
              </div>
            </div>

            <div className='home-hero-inner relative z-[1] mx-auto flex max-w-5xl flex-col items-center px-4 pt-20 pb-10 text-center'>
              <h1 className='home-hero-title max-w-3xl text-4xl font-bold leading-tight text-semi-color-text-0 md:text-5xl lg:text-6xl'>
                <span
                  className='home-hero-title-line home-hero-title-line-1'
                  aria-label={t('统一的')}
                >
                  {Array.from(t('统一的')).map((ch, idx) => (
                    <span
                      key={`hero-title-l1-${idx}`}
                      className='home-hero-title-char'
                      style={{ '--char-i': idx }}
                      aria-hidden='true'
                    >
                      {ch === ' ' ? ' ' : ch}
                    </span>
                  ))}
                </span>
                <br />
                <span className='home-hero-title-line home-hero-title-line-2'>
                  <span className='home-title-gradient home-title-gradient-shimmer'>
                    {t('大模型接口网关')}
                  </span>
                  <span
                    className='home-hero-title-sweep'
                    aria-hidden='true'
                  />
                </span>
              </h1>

              <p className='home-hero-subtitle mt-4 max-w-xl text-base text-semi-color-text-1 md:text-lg'>
                {t('一个 Endpoint，兼容 OpenAI、Claude、Gemini 等主流模型的 API')}
              </p>

              {/* API endpoint bar */}
              <div className='home-hero-endpoint mt-8 w-full' style={{ maxWidth: 880 }}>
                <div className='home-endpoint-shell flex items-center gap-2 p-1.5 pl-3 sm:gap-3 sm:pl-4'>
                  <div className='flex shrink-0 items-center gap-2'>
                    <span className='relative flex h-2 w-2'>
                      <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-semi-color-success opacity-60' />
                      <span className='relative inline-flex h-2 w-2 rounded-full bg-semi-color-success' />
                    </span>
                    <span className='font-mono text-[11px] font-semibold tracking-wider text-semi-color-success sm:text-xs'>
                      POST
                    </span>
                  </div>
                  <span
                    className='home-endpoint-divider h-5 w-px shrink-0'
                    aria-hidden='true'
                  />
                  <code className='min-w-0 flex-1 truncate text-left font-mono text-[12px] text-semi-color-text-1 sm:text-[13px]'>
                    <span className='text-semi-color-text-2'>
                      {serverAddress}
                    </span>
                    <span className='home-endpoint-path font-medium'>
                      {ENDPOINT_PATH}
                    </span>
                  </code>
                  <Button
                    type='tertiary'
                    theme='borderless'
                    className='shrink-0'
                    onClick={handleCopyEndpoint}
                    aria-label={copied ? t('已复制到剪切板') : t('复制')}
                    icon={
                      copied ? (
                        <Check size={16} className='text-semi-color-success' />
                      ) : (
                        <Copy size={16} />
                      )
                    }
                  />
                </div>
              </div>

              {/* 操作按钮 */}
              <div className='home-hero-actions mt-5 flex flex-wrap items-center justify-center gap-3'>
                <Link to='/console'>
                  <Button
                    theme='solid'
                    type='primary'
                    size={isMobile ? 'default' : 'large'}
                    className='!rounded-xl px-6'
                    icon={<KeyRound size={16} />}
                  >
                    {t('获取密钥')}
                  </Button>
                </Link>
                {isDemoSiteMode && statusState?.status?.version ? (
                  <Button
                    size={isMobile ? 'default' : 'large'}
                    className='!rounded-xl px-6'
                    icon={<Github size={16} />}
                    onClick={() =>
                      window.open(
                        'https://github.com/QuantumNous/new-api',
                        '_blank',
                      )
                    }
                  >
                    {statusState.status.version}
                  </Button>
                ) : (
                  <Button
                    size={isMobile ? 'default' : 'large'}
                    className='!rounded-xl px-6'
                    icon={<BookOpen size={16} />}
                    onClick={() => window.open(docsLink, '_blank')}
                  >
                    {t('文档')}
                  </Button>
                )}
              </div>
            </div>
          </section>

          {/* ==================== 供应商网格 ==================== */}
          <section className='relative z-[1] px-4 py-6 md:py-8'>
            <div className='mx-auto w-full' style={{ maxWidth: 1280 }}>
              <div className='home-provider-heading-block'>
                <h2 className='text-center text-xl font-bold tracking-tight text-semi-color-text-0 md:text-2xl'>
                  {t('支持众多的大模型供应商')}
                </h2>
                <p className='mx-auto mt-2 max-w-xl text-center text-sm text-semi-color-text-2'>
                  {t('一套 API，接入所有主流大模型供应商')}
                </p>
              </div>

              {/* 第一行：4 个 featured 大卡 */}
              <div className='home-provider-featured-grid'>
                {FEATURED_PROVIDERS.map((provider, idx) => (
                  <div
                    key={provider.name}
                    className={`home-provider-featured-card ${provider.accentClass}`}
                    style={{ '--card-i': idx }}
                  >
                    <span className='home-provider-featured-icon'>
                      {provider.icon}
                    </span>
                    <span className='home-provider-featured-name'>
                      {provider.name}
                    </span>
                    <span className='home-provider-pill'>
                      <span className='home-provider-dot' />
                      <span className='truncate'>{t(provider.compatLabel)}</span>
                    </span>
                  </div>
                ))}
              </div>

              {/* 第二、三行：小卡 6 列 × 2 行（含 30+ 占位卡） */}
              <div className='home-provider-secondary-grid'>
                {SECONDARY_PROVIDERS.map((provider, idx) => (
                  <div
                    key={provider.name}
                    className='home-provider-secondary-card'
                    style={{ '--card-i': idx }}
                  >
                    <span className='home-provider-secondary-icon'>
                      {provider.icon}
                    </span>
                    <div className='home-provider-secondary-content'>
                      <div className='home-provider-secondary-name'>
                        {provider.name}
                      </div>
                      <div className='home-provider-pill'>
                        <span className='home-provider-dot' />
                        <span className='truncate'>{t(provider.compatLabel)}</span>
                      </div>
                    </div>
                  </div>
                ))}
                <div
                  className='home-provider-more-card'
                  style={{ '--card-i': SECONDARY_PROVIDERS.length }}
                >
                  <span className='home-provider-secondary-icon'>30+</span>
                  <div className='home-provider-secondary-content'>
                    <div className='home-provider-secondary-name'>
                      {t('更多供应商')}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* ==================== 功能条 ==================== */}
          <section className='relative z-[1] px-4 pb-16 md:pb-20'>
            <div
              className='home-feature-strip home-feature-strip-block mx-auto w-full'
              style={{ maxWidth: 1520 }}
            >
              <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5'>
                {features.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div
                      key={item.title}
                      className='home-feature-item flex items-center gap-3 border-b border-semi-color-border px-5 py-5 last:border-b-0 lg:border-b-0 lg:border-r lg:last:border-r-0'
                    >
                      <span className='home-feature-icon flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-semi-color-border bg-semi-color-bg-0'>
                        <Icon size={18} style={{ color: item.color }} />
                      </span>
                      <div className='min-w-0 text-left'>
                        <div className='text-sm font-semibold tracking-tight text-semi-color-text-0'>
                          {item.title}
                        </div>
                        <Text type='tertiary' className='text-xs leading-snug'>
                          {item.desc}
                        </Text>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          </section>
        </div>
      ) : (
        <div className='classic-page-fill overflow-x-hidden w-full'>
          {homePageContent.startsWith('https://') ? (
            <iframe
              src={homePageContent}
              className='w-full h-full border-none'
            />
          ) : (
            <div
              className='mt-[60px]'
              dangerouslySetInnerHTML={{ __html: homePageContent }}
            />
          )}
        </div>
      )}
    </div>
  );
};

export default Home;
