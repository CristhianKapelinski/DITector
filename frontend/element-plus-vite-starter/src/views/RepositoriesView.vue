<template>
    <div class="input-with-search">
        <el-input id="input-1" v-model="searchKeyword" placeholder="namespace, name or description of repository" clearable>
            <template #append>
                <el-button id="search-1" type="primary" @click="handleSearchRepositories">Search</el-button>
            </template>
        </el-input>
    </div>
    <el-table
        :data="repositoriesData"
        highlight-current-row
        stripe
        table-layout="fixed"
        style="width: 100%"
        max-height="700"
    >
        <!--        可收缩展开内容-->
        <el-table-column fixed type="expand">
            <template #default="repoProps">
                <div>
                    <el-table
                        id="expanded-tags-table"
                        highlight-current-row
                        :data="repoProps.row.tags"
                        :row-class-name="tableRowClassName"
                    >
                        <el-table-column fixed label="" width="50" />
                        <el-table-column fixed type="expand">
                            <template #default="tagProps">
                                <div>
                                    <el-table
                                        id="expanded-images-table"
                                        highlight-current-row
                                        :data="tagProps.row.images"
                                        :row-class-name="tableRowClassName"
                                    >
                                        <el-table-column label="" width="150" />
                                        <el-table-column prop="colId" label="Index" align="center" width="200" />
                                        <el-table-column prop="architecture" label="Architecture" align="center" width="200" />
                                        <el-table-column prop="variant" label="Variant" align="center" width="400" />
                                        <el-table-column prop="digest" label="Digest" width="1000" />
                                    </el-table>
                                </div>
                            </template>
                        </el-table-column>
                        <el-table-column prop="colId" label="Index" align="center" width="80" />
                        <el-table-column prop="tag_name" label="Tag Name" align="center" width="200" />
                        <el-table-column prop="tag_last_pulled" label="Last Updated" align="center" width="240" />
                        <el-table-column prop="last_updater_username" label="Last Updater" align="center" show-overflow-tooltip width="200" />
                        <el-table-column prop="tag_last_pulled" label="Last Pulled" align="center" width="240" />
                        <el-table-column prop="tag_last_pushed" label="Last Pushed" align="center" width="240" />
                        <el-table-column prop="media_type" label="Media Type" show-overflow-tooltip align="center" width="400" />
                        <el-table-column prop="content_type" label="Content Type" align="center" width="150" />
                    </el-table>
                </div>
            </template>
        </el-table-column>
        <el-table-column fixed prop="namespace" label="Namespace" align="center" show-overflow-tooltip width="200" />
        <el-table-column fixed prop="name" label="Name" align="center" show-overflow-tooltip width="200" />
        <el-table-column prop="user" label="User" align="center" show-overflow-tooltip width="200" />
        <el-table-column prop="repository_type" label="Repo Type" align="center" width="100" />
        <el-table-column prop="description" label="Description" show-overflow-tooltip width="300" />
        <el-table-column prop="star_count" label="Star Count" align="center" width="100" />
        <el-table-column prop="pull_count" label="Pull Count" align="center" width="125" />
        <el-table-column prop="date_registered" label="Date Registered" align="center" width="240" />
        <el-table-column prop="last_updated" label="Last Updated" align="center" width="240" />
        <el-table-column prop="full_description" label="Full Description" show-overflow-tooltip width="600" />
    </el-table>
    <div class="pagination-bottom">
        <el-pagination
            :currentPage="currentPage"
            :page-sizes="[10, 15, 20]"
            :page-size="pageSize"
            layout=" prev, pager, next, jumper, sizes, total, "
            :total="totalPages"
            @size-change="handleSizeChange"
            @current-change="handleCurrentChange"
            align="center"
        />
    </div>
</template>

<script lang="ts" setup>
import { ref } from 'vue';
import axios from 'axios';

const tableRowClassName = ({row, rowIndex}) => {
    row.colId = rowIndex + 1;
};

const currentPage = ref(1);
const pageSize = ref(20);
const totalCnt = ref(0);    // total count of documents in response
const totalPages = ref(0);  // total count of pages (totalCnt/pageSize + 1)
const searchKeyword = ref('');
const repositoriesData = ref([]);

function handleSearchRepositories() {
    // console.log('search repositories');
    // reset to page 1 before every search
    currentPage.value = 1;
    fetchRepositoriesData();
}

function getRepositoriesData(search, page, pageSize) {
    // axios get images data responsed from backend API
    axios.get('http://10.10.21.122:23434/repositories', {
        params: {
            search: search,
            page: page,
            page_size: pageSize
        }
    }).then(response => {
        repositoriesData.value = response.data['results'];
        // console.log(imagesData.value);
        // console.log(response.data);
    })
    .catch(error => {
        console.log(error);
    });
}

function handleCurrentChange(val: number) {
    currentPage.value = val;
    console.log(currentPage.value);
    fetchRepositoriesData();
}

function handleSizeChange(val: number) {
    currentPage.value = 1;
    // change pageSize
    pageSize.value = val;
    // recalculate totalPages
    recalculateTotalPages();
    console.log(pageSize.value);
    fetchRepositoriesData();
}

// recalculate TotalPages after totalCnt or pageSize changed
function recalculateTotalPages() {
    if (totalCnt.value === 0) {
        totalPages.value = 0;
    } else {
        totalPages.value = Math.floor(totalCnt.value / pageSize.value + 1);
    }
}

// fetch repositories data from backend with searchKeyword, currentPage and pageSize
function fetchRepositoriesData() {
    getRepositoriesData(searchKeyword.value, currentPage.value, pageSize.value);
}

fetchRepositoriesData();
</script>

<style scoped>
.input-with-search {
    float: right;
    width: 45%;
}

.pagination-bottom {
    margin-top: 20;
    display: flex;
    justify-content: center;
}
</style>