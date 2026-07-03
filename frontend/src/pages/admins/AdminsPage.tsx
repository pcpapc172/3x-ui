import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Col, Form, Input, InputNumber, Layout, Modal, Popconfirm, Row, Space, Table, Tag, Tooltip, message } from 'antd';
import { DeleteOutlined, EditOutlined, PlusOutlined, ReloadOutlined, CopyOutlined } from '@ant-design/icons';

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
  const [resellers, setResellers] = useState<ResellerInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingReseller, setEditingReseller] = useState<ResellerInfo | null>(null);
  const [form] = Form.useForm();

  const generateRandom = useCallback(() => {
    const chars = 'abcdefghijklmnopqrstuvwxyz0123456789';
    const pick = (len: number) => Array.from({ length: len }, () => chars[Math.floor(Math.random() * chars.length)]).join('');
    form.setFieldsValue({ username: `rs-${pick(8)}`, password: pick(16) });
  }, [form]);

  const fetchResellers = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await HttpUtil.get<Msg<ResellerInfo[]>>('/panel/api/resellers/list');
      if (resp.success && resp.obj) {
        setResellers(resp.obj);
      }
    } catch {
      // handled by HttpUtil
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchResellers();
  }, [fetchResellers]);

  const handleAdd = () => {
    setEditingReseller(null);
    form.resetFields();
    form.setFieldsValue({ usageLimit: 0 });
    setModalOpen(true);
  };

  const handleEdit = (record: ResellerInfo) => {
    setEditingReseller(record);
    form.setFieldsValue({
      username: record.username,
      password: '',
      usageLimit: record.usageLimit / (1024 * 1024 * 1024),
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
        message.success(`Reset usage, re-enabled ${resp.obj?.reEnabled ?? 0} clients`);
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
      title: 'Clients',
      dataIndex: 'clientCount',
      key: 'clientCount',
      width: 80,
    },
    {
      title: 'Upload',
      dataIndex: 'usageUp',
      key: 'usageUp',
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'Download',
      dataIndex: 'usageDown',
      key: 'usageDown',
      render: (v: number) => formatBytes(v),
    },
    {
      title: 'Limit',
      dataIndex: 'usageLimit',
      key: 'usageLimit',
      render: (v: number) => v === 0 ? 'Unlimited' : formatBytes(v),
    },
    {
      title: 'Status',
      key: 'status',
      render: (_: unknown, record: ResellerInfo) => {
        const overQuota = record.usageLimit > 0 && (record.usageUp + record.usageDown) >= record.usageLimit;
        return overQuota
          ? <Tag color="red">Over Quota</Tag>
          : <Tag color="green">Active</Tag>;
      },
    },
    {
      title: t('table actions', 'Actions'),
      key: 'actions',
      width: 200,
      render: (_: unknown, record: ResellerInfo) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          <Popconfirm title="Reset usage and re-enable all clients?" onConfirm={() => handleResetUsage(record.id)}>
            <Button size="small" icon={<ReloadOutlined />} />
          </Popconfirm>
          <Popconfirm title="Delete this reseller?" onConfirm={() => handleDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <Layout.Content style={{ padding: 24 }}>
      <Card
        title="Resellers"
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={handleAdd}>
            Add Reseller
          </Button>
        }
      >
        <Table
          dataSource={resellers}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={false}
        />
      </Card>

      <Modal
        title={editingReseller ? 'Edit Reseller' : 'Add Reseller'}
        open={modalOpen}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
        okText={editingReseller ? 'Update' : 'Create'}
      >
        <Form form={form} layout="vertical">
          {!editingReseller && (
            <Form.Item>
              <Tooltip title="Generate random username and password">
                <Button icon={<CopyOutlined />} onClick={generateRandom}>Generate Random</Button>
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
            <Input.Password placeholder={editingReseller ? 'Leave blank to keep current' : ''} />
          </Form.Item>
          <Form.Item name="usageLimit" label="Usage Limit (GB)">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </Layout.Content>
  );
}
