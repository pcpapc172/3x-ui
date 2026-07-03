import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Layout,
  Result,
  Row,
  Spin,
  Statistic,
} from 'antd';
import {
  CloudDownloadOutlined,
  CloudUploadOutlined,
  CloudSyncOutlined,
  TeamOutlined,
  CheckCircleOutlined,
  DatabaseOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import type { Msg } from '@/utils';

interface ResellerOverview {
  id: number;
  username: string;
  role: string;
  usageLimit: number;
  usageUp: number;
  usageDown: number;
  remaining: number;
  clientCount: number;
  enabledCount: number;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export default function ResellerOverviewPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [overview, setOverview] = useState<ResellerOverview | null>(null);
  const [fetched, setFetched] = useState(false);
  const [fetchError, setFetchError] = useState('');

  const pageClass = useMemo(() => {
    const classes = ['reseller-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const fetchOverview = useCallback(async () => {
    setFetchError('');
    try {
      const resp = await HttpUtil.get<Msg<ResellerOverview>>('/panel/api/user/overview');
      if (resp.success && resp.obj) {
        setOverview(resp.obj);
      }
    } catch {
      setFetchError(t('somethingWentWrong'));
    } finally {
      setFetched(true);
    }
  }, [t]);

  useEffect(() => { fetchOverview(); }, [fetchOverview]);

  const totalUsage = overview ? overview.usageUp + overview.usageDown : 0;
  const usagePercent = overview && overview.usageLimit > 0
    ? Math.min(100, (totalUsage / overview.usageLimit) * 100)
    : 0;

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />

        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!fetched} delay={200} description={t('loading')} size="large">
              {!fetched ? (
                <div className="loading-spacer" />
              ) : fetchError ? (
                <Result
                  status="error"
                  title={t('somethingWentWrong')}
                  subTitle={fetchError}
                  extra={<Button type="primary" onClick={fetchOverview}>{t('refresh')}</Button>}
                />
              ) : overview && (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.reseller.sent', 'Sent')}
                            value={formatBytes(overview.usageUp)}
                            prefix={<CloudUploadOutlined style={{ color: 'var(--ant-color-info)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.reseller.received', 'Received')}
                            value={formatBytes(overview.usageDown)}
                            prefix={<CloudDownloadOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.reseller.remaining', 'Remaining')}
                            value={overview.usageLimit > 0 ? formatBytes(overview.remaining) : t('pages.reseller.unlimited', 'Unlimited')}
                            prefix={<CloudSyncOutlined style={{ color: overview.usageLimit > 0 && overview.remaining < overview.usageLimit * 0.1 ? 'var(--ant-color-error)' : 'var(--ant-color-warning)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.reseller.totalUsage', 'Total Usage')}
                            value={formatBytes(totalUsage)}
                            suffix={overview.usageLimit > 0 ? `/ ${formatBytes(overview.usageLimit)}` : ''}
                            prefix={<DatabaseOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card size="small" hoverable>
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.reseller.totalClients', 'Total Clients')}
                            value={overview.clientCount}
                            prefix={<TeamOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.reseller.activeClients', 'Active Clients')}
                            value={overview.enabledCount}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.reseller.quotaUsed', 'Quota Used')}
                            value={overview.usageLimit > 0 ? `${usagePercent.toFixed(1)}%` : '-'}
                            prefix={<DatabaseOutlined style={{ color: usagePercent > 90 ? 'var(--ant-color-error)' : usagePercent > 70 ? 'var(--ant-color-warning)' : 'var(--ant-color-info)' }} />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
