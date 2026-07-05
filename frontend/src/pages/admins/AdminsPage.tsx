import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  ConfigProvider,
  Form,
  Input,
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
  Switch,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd';
import {
  DeleteOutlined,
  EditOutlined,
  PlusOutlined,
  ReloadOutlined,
  CopyOutlined,
  TeamOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloudDownloadOutlined,
} from '@ant-design/icons';

import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';
import { HttpUtil } from '@/utils';
import type { Msg } from '@/utils';

interface ResellerInfo {
  id: number;
  username: string;
  role: string;
  usageLimit: number;
  usageUp: number;
  usageDown: number;
  clientCount: number;
  enabled: boolean;
  allowedInboundsMode: string;
  allowedInboundIds: number[];
}

interface InboundOption {
  id: number;
  remark?: string;
  tag?: string;
  protocol?: string;
  port?: number;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export default function AdminsPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [resellers, setResellers] = useState<ResellerInfo[]>([]);
  const [fetched, setFetched] = useState(false);
  const [loading, setLoading] = useState(false);
  const [fetchError, setFetchError] = useState('');
  const [modalOpen, setModalOpen] = useState(false);
  const [editingReseller, setEditingReseller] = useState<ResellerInfo | null>(null);
  const [inbounds, setInbounds] = useState<InboundOption[]>([]);
  const [inboundMode, setInboundMode] = useState<string>('all');
  const [form] = Form.useForm();

  const pageClass = useMemo(() => {
    const classes = ['admins-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const totals = useMemo(() => {
    let active = 0;
    let totalUp = 0;
    let totalDown = 0;
    let totalClients = 0;
    for (const r of resellers) {
      const overQuota = r.usageLimit > 0 && (r.usageUp + r.usageDown) >= r.usageLimit;
      if (!overQuota) active++;
      totalUp += r.usageUp;
      totalDown += r.usageDown;
      totalClients += r.clientCount;
    }
    return { total: resellers.length, active, totalUp, totalDown, totalClients };
  }, [resellers]);

  const generateRandom = useCallback(() => {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    const pick = (len: number) => Array.from({ length: len }, () => chars[Math.floor(Math.random() * chars.length)]).join('');
    form.setFieldsValue({ username: `rs-${pick(8)}`, password: pick(16) });
  }, [form]);

  const fetchResellers = useCallback(async () => {
    setLoading(true);
    setFetchError('');
    try {
      const [resp, inbResp] = await Promise.all([
        HttpUtil.get<Msg<ResellerInfo[]>>('/panel/api/resellers/list'),
        HttpUtil.get<Msg<InboundOption[]>>('/panel/api/inbounds/options'),
      ]);
      if (resp.success && resp.obj) {
        setResellers(resp.obj);
      }
      if (inbResp.success && inbResp.obj) {
        setInbounds(inbResp.obj);
      }
    } catch {
      setFetchError(t('somethingWentWrong'));
    } finally {
      setLoading(false);
      setFetched(true);
    }
  }, [t]);

  useEffect(() => {
    fetchResellers();
  }, [fetchResellers]);

  const handleAdd = () => {
    setEditingReseller(null);
    setInboundMode('all');
    form.resetFields();
    form.setFieldsValue({ usageLimit: 0, allowedInboundsMode: 'all', allowedInboundIds: [] });
    setModalOpen(true);
  };

  const handleEdit = (record: ResellerInfo) => {
    setEditingReseller(record);
    const mode = record.allowedInboundsMode || 'all';
    setInboundMode(mode);
    form.setFieldsValue({
      username: record.username,
      password: '',
      usageLimit: record.usageLimit / (1024 * 1024 * 1024),
      allowedInboundsMode: mode,
      allowedInboundIds: record.allowedInboundIds || [],
    });
    setModalOpen(true);
  };

  const handleDelete = async (id: number) => {
    try {
      const resp = await HttpUtil.post<Msg<unknown>>(`/panel/api/resellers/del/${id}`);
      if (resp.success) {
        fetchResellers();
      }
    } catch {
      // handled by HttpUtil
    }
  };

  const handleResetUsage = async (id: number) => {
    try {
      const resp = await HttpUtil.post<Msg<{ reEnabled: number }>>(`/panel/api/resellers/resetUsage/${id}`);
      if (resp.success) {
        messageApi.success(`Reset usage, re-enabled ${resp.obj?.reEnabled ?? 0} clients`);
        fetchResellers();
      }
    } catch {
      // handled by HttpUtil
    }
  };

  const handleToggleEnable = async (id: number, checked: boolean) => {
    try {
      const resp = await HttpUtil.post<Msg<{ enabled: boolean }>>(`/panel/api/resellers/toggleEnable/${id}`);
      if (resp.success) {
        messageApi.success(checked ? 'Reseller enabled' : 'Reseller disabled');
        fetchResellers();
      }
    } catch {
      // handled by HttpUtil
    }
  };

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      const usageLimitBytes = (values.usageLimit || 0) * 1024 * 1024 * 1024;
      const payload = {
        username: values.username,
        password: values.password || '',
        usageLimit: usageLimitBytes,
        allowedInboundsMode: values.allowedInboundsMode || 'all',
        allowedInboundIds: values.allowedInboundsMode === 'select' ? (values.allowedInboundIds || []) : [],
      };
      let resp: Msg<unknown>;
      if (editingReseller) {
        resp = await HttpUtil.post<Msg<unknown>>(`/panel/api/resellers/update/${editingReseller.id}`, payload);
      } else {
        resp = await HttpUtil.post<Msg<unknown>>('/panel/api/resellers/add', payload);
      }
      if (resp.success) {
        setModalOpen(false);
        fetchResellers();
      }
    } catch {
      // validation or network error
    }
  };

  const columns = [
    {
      title: 'ID',
      dataIndex: 'id',
      key: 'id',
      width: 60,
    },
    {
      title: t('pages.settings.username', 'Username'),
      dataIndex: 'username',
      key: 'username',
    },
    {
      title: t('pages.clients.title', 'Clients'),
      dataIndex: 'clientCount',
      key: 'clientCount',
      width: 80,
    },
    {
      title: t('pages.inbounds.up', 'Upload'),
      dataIndex: 'usageUp',
      key: 'usageUp',
      render: (v: number) => formatBytes(v),
    },
    {
      title: t('pages.inbounds.down', 'Download'),
      dataIndex: 'usageDown',
      key: 'usageDown',
      render: (v: number) => formatBytes(v),
    },
    {
      title: t('pages.settings.limit', 'Limit'),
      dataIndex: 'usageLimit',
      key: 'usageLimit',
      render: (v: number) => v === 0 ? t('pages.settings.unlimited', 'Unlimited') : formatBytes(v),
    },
    {
      title: t('pages.settings.status', 'Status'),
      key: 'status',
      width: 140,
      render: (_: unknown, record: ResellerInfo) => {
        const overQuota = record.usageLimit > 0 && (record.usageUp + record.usageDown) >= record.usageLimit;
        return (
          <Space size="small">
            <Switch
              size="small"
              checked={record.enabled}
              onChange={(checked) => handleToggleEnable(record.id, checked)}
            />
            {overQuota
              ? <Tag color="error">{t('pages.settings.overQuota', 'Over Quota')}</Tag>
              : <Tag color="success">{t('pages.settings.active', 'Active')}</Tag>}
          </Space>
        );
      },
    },
    {
      title: t('table actions', 'Actions'),
      key: 'actions',
      width: 200,
      render: (_: unknown, record: ResellerInfo) => (
        <Space>
          <Tooltip title={t('edit', 'Edit')}>
            <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Popconfirm title={t('pages.admins.resetUsageConfirm', 'Reset usage and re-enable all clients?')} onConfirm={() => handleResetUsage(record.id)}>
            <Tooltip title={t('pages.admins.resetUsage', 'Reset Usage')}>
              <Button size="small" icon={<ReloadOutlined />} />
            </Tooltip>
          </Popconfirm>
          <Popconfirm title={t('pages.admins.deleteConfirm', 'Delete this reseller?')} onConfirm={() => handleDelete(record.id)}>
            <Tooltip title={t('delete', 'Delete')}>
              <Button size="small" danger icon={<DeleteOutlined />} />
            </Tooltip>
          </Popconfirm>
        </Space>
      ),
    },
  ];

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
                  extra={<Button type="primary" onClick={fetchResellers}>{t('refresh')}</Button>}
                />
              ) : (
                <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
                  <Col span={24}>
                    <Card size="small" hoverable className="summary-card">
                      <Row gutter={[16, isMobile ? 16 : 12]}>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.totalResellers', 'Total Resellers')}
                            value={totals.total}
                            prefix={<TeamOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.activeResellers', 'Active')}
                            value={totals.active}
                            prefix={<CheckCircleOutlined style={{ color: 'var(--ant-color-success)' }} />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.totalClients', 'Total Clients')}
                            value={totals.totalClients}
                            prefix={<ExclamationCircleOutlined />}
                          />
                        </Col>
                        <Col xs={12} sm={12} md={6}>
                          <Statistic
                            title={t('pages.admins.totalUsage', 'Total Usage')}
                            value={formatBytes(totals.totalUp + totals.totalDown)}
                            prefix={<CloudDownloadOutlined />}
                          />
                        </Col>
                      </Row>
                    </Card>
                  </Col>

                  <Col span={24}>
                    <Card
                      size="small"
                      hoverable
                      title={t('pages.admins.resellers', 'Resellers')}
                      extra={
                        <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
                          {isMobile ? '' : t('pages.admins.addReseller', 'Add Reseller')}
                        </Button>
                      }
                    >
                      <Table
                        dataSource={resellers}
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

        <Modal
          title={editingReseller ? t('pages.admins.editReseller', 'Edit Reseller') : t('pages.admins.addReseller', 'Add Reseller')}
          open={modalOpen}
          onOk={handleSubmit}
          onCancel={() => setModalOpen(false)}
          okText={editingReseller ? t('update', 'Update') : t('create', 'Create')}
        >
          <Form form={form} layout="vertical">
            {!editingReseller && (
              <Form.Item>
                <Tooltip title={t('pages.admins.generateRandom', 'Generate random username and password')}>
                  <Button icon={<CopyOutlined />} onClick={generateRandom}>{t('pages.admins.generateRandom', 'Generate Random')}</Button>
                </Tooltip>
              </Form.Item>
            )}
            <Form.Item
              name="username"
              label={t('pages.settings.username', 'Username')}
              rules={[{ required: true }]}
            >
              <Input disabled={!!editingReseller} />
            </Form.Item>
            <Form.Item
              name="password"
              label={t('pages.settings.password', 'Password')}
              rules={editingReseller ? [] : [{ required: true }]}
            >
              <Input.Password placeholder={editingReseller ? t('pages.admins.leaveBlank', 'Leave blank to keep current') : ''} />
            </Form.Item>
            <Form.Item name="usageLimit" label={t('pages.admins.usageLimitGB', 'Usage Limit (GB)')}>
              <InputNumber min={0} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="allowedInboundsMode" label={t('pages.admins.allowedInboundsMode', 'Allowed Inbounds')}>
              <Select
                onChange={(val) => {
                  setInboundMode(val);
                  if (val === 'all') {
                    form.setFieldsValue({ allowedInboundIds: [] });
                  }
                }}
                options={[
                  { value: 'all', label: t('pages.admins.inboundAll', 'All Inbounds') },
                  { value: 'select', label: t('pages.admins.inboundSelect', 'Select Inbounds') },
                ]}
              />
            </Form.Item>
            {inboundMode === 'select' && (
              <Form.Item name="allowedInboundIds" label={t('pages.admins.selectInbounds', 'Select Inbounds')}>
                <Select
                  mode="multiple"
                  placeholder={t('pages.admins.selectInboundsPlaceholder', 'Select inbounds this reseller can access')}
                  options={inbounds.map((ib) => ({
                    value: ib.id,
                    label: `${ib.remark || ib.tag || ib.id} (${ib.protocol}:${ib.port})`,
                  }))}
                />
              </Form.Item>
            )}
          </Form>
        </Modal>
      </Layout>
    </ConfigProvider>
  );
}
