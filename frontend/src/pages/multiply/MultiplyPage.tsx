import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  InputNumber,
  Layout,
  Modal,
  Popconfirm,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Statistic,
  Table,
  message,
} from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ApartmentOutlined,
  NumberOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import type { Msg } from '@/utils';

interface MultiplyEntry {
  id: number;
  inboundId: number;
  inboundTag: string;
  rate: number;
}

interface InboundOption {
  id: number;
  remark?: string;
  tag?: string;
  protocol?: string;
  port?: number;
}

export default function MultiplyPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [entries, setEntries] = useState<MultiplyEntry[]>([]);
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [fetched, setFetched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState('');
  const [selectedInbound, setSelectedInbound] = useState<number | null>(null);
  const [rate, setRate] = useState<number>(1);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [editingEntry, setEditingEntry] = useState<MultiplyEntry | null>(null);
  const [editRate, setEditRate] = useState<number>(1);

  const pageClass = useMemo(() => {
    const classes = ['multiply-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const totals = useMemo(() => {
    let maxRate = 0;
    let avgRate = 0;
    for (const e of entries) {
      if (e.rate > maxRate) maxRate = e.rate;
      avgRate += e.rate;
    }
    if (entries.length > 0) avgRate /= entries.length;
    return { total: entries.length, maxRate, avgRate };
  }, [entries]);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setFetchError('');
    try {
      const [multResp, inbResp] = await Promise.all([
        HttpUtil.get<Msg<MultiplyEntry[]>>('/panel/api/multiply/list'),
        HttpUtil.get<Msg<InboundOption[]>>('/panel/api/inbounds/options'),
      ]);
      if (multResp.success && multResp.obj) setEntries(multResp.obj);
      if (inbResp.success && inbResp.obj) setInbounds(inbResp.obj);
    } catch {
      setFetchError(t('somethingWentWrong'));
    } finally {
      setLoading(false);
      setFetched(true);
    }
  }, [t]);

  useEffect(() => { fetchData(); }, [fetchData]);

  const handleAdd = async () => {
    if (!selectedInbound) return;
    const resp = await HttpUtil.post('/panel/api/multiply/set', {
      inboundId: selectedInbound,
      rate,
    });
    if (resp.success) {
      setSelectedInbound(null);
      setRate(1);
      fetchData();
    }
  };

  const handleDelete = async (id: number) => {
    const resp = await HttpUtil.post(`/panel/api/multiply/del/${id}`);
    if (resp.success) fetchData();
  };

  const handleEdit = (entry: MultiplyEntry) => {
    setEditingEntry(entry);
    setEditRate(entry.rate);
    setEditModalOpen(true);
  };

  const handleEditSubmit = async () => {
    if (!editingEntry) return;
    const resp = await HttpUtil.post('/panel/api/multiply/set', {
      inboundId: editingEntry.inboundId,
      rate: editRate,
    });
    if (resp.success) {
      setEditModalOpen(false);
      setEditingEntry(null);
      fetchData();
    }
  };

  const columns = [
    {
      title: t('pages.multiply.inbound', 'Inbound'),
      dataIndex: 'inboundTag',
      key: 'inboundTag',
      render: (tag: string) => tag || 'Unknown',
    },
    {
      title: t('pages.multiply.rate', 'Rate'),
      dataIndex: 'rate',
      key: 'rate',
      render: (r: number) => <strong>{r}x</strong>,
    },
    {
      title: '',
      key: 'actions',
      width: 100,
      render: (_: unknown, record: MultiplyEntry) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Popconfirm title={t('pages.multiply.removeConfirm', 'Remove this multiplier?')} onConfirm={() => handleDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const usedInboundIds = new Set(entries.map((e) => e.inboundId));
  const availableInbounds = inbounds.filter((ib) => !usedInboundIds.has(ib.id));

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
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
                  extra={<Button type="primary" onClick={fetchData}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.multiply.totalMultipliers', 'Total Multipliers')}
                            value={totals.total}
                            prefix={<ApartmentOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.multiply.maxRate', 'Max Rate')}
                            value={totals.maxRate > 0 ? `${totals.maxRate}x` : '-'}
                            prefix={<NumberOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={8}>
                          <Statistic
                            title={t('pages.multiply.avgRate', 'Avg Rate')}
                            value={totals.avgRate > 0 ? `${totals.avgRate.toFixed(1)}x` : '-'}
                            prefix={<NumberOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={t('pages.multiply.trafficMultiplier', 'Traffic Multiplier')}
                      extra={
                        <Space>
                          <Select
                            style={{ width: isMobile ? 140 : 200 }}
                            placeholder={t('pages.multiply.selectInbound', 'Select inbound')}
                            value={selectedInbound}
                            onChange={setSelectedInbound}
                            options={availableInbounds.map((ib) => ({
                              value: ib.id,
                              label: `${ib.remark || ib.tag || ib.id} (${ib.protocol}:${ib.port})`,
                            }))}
                          />
                          <InputNumber
                            min={0.1}
                            step={0.1}
                            value={rate}
                            onChange={(v) => setRate(v ?? 1)}
                            addonAfter="x"
                            style={{ width: 100 }}
                          />
                          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd} disabled={!selectedInbound}>
                            {isMobile ? '' : t('add', 'Add')}
                          </Button>
                        </Space>
                      }
                    >
                      <Table
                        dataSource={entries}
                        columns={columns}
                        rowKey="id"
                        loading={loading}
                        pagination={false}
                        size="small"
                      />
                    </Card>
                  </Col>
                </Row>
              )}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>

      <Modal
        title={t('pages.multiply.editMultiplier', 'Edit Multiplier')}
        open={editModalOpen}
        onOk={handleEditSubmit}
        onCancel={() => { setEditModalOpen(false); setEditingEntry(null); }}
        okText={t('update', 'Update')}
      >
        <div style={{ marginBottom: 8 }}>
          <strong>{t('pages.multiply.inbound', 'Inbound')}:</strong> {editingEntry?.inboundTag || 'Unknown'}
        </div>
        <InputNumber
          min={0.1}
          step={0.1}
          value={editRate}
          onChange={(v) => setEditRate(v ?? 1)}
          addonAfter="x"
          style={{ width: '100%' }}
        />
      </Modal>
    </ConfigProvider>
  );
}
